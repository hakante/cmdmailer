package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"

	"gopkg.in/gcfg.v1"

	"terelius.dev/go/mail"
)

type bufferCSSWriter struct {
	buffer           *bytes.Buffer
	startTag, endTag string
}

func (b bufferCSSWriter) Write(p []byte) (n int, err error) {
	if b.buffer.Len() < 1000000 {
		b.buffer.WriteString(b.startTag)
		n, err = (b.buffer).Write([]byte(html.EscapeString(string(p))))
		b.buffer.WriteString(b.endTag)
	}
	// Fake always successful to not break other writers.
	return len(p), nil
}

func main() {
	// Parse options
	var fromFlag = flag.String("from", "", "Author email address")
	var toFlag = flag.String("to", "", "Recipient email address")
	var subjectFlag = flag.String("subject", "", "Email subject (optional)")
	var addressFlag = flag.String("host", "", "Email server address")
	var userFlag = flag.String("user", "", "Email server user name")
	var passwordFlag = flag.String("password", "", "Email server password")
	var helpFlag = flag.Bool("help", false, "Print this help message and exit")
	var mailOutputFlag = flag.Bool("mail-output", true, "Include STDOUT/STDERR in email")
	flag.Parse()

	if *helpFlag {
		fmt.Print("Usage: cmdmailer [-options] command\n  to execute the command\n where the options are:\n")
		flag.PrintDefaults()
		fmt.Print("\nwhere default options are read from the file ~/cmdmailer.conf.\nA sample configuration file is:\n\n[message]\nfrom = from@example.com\nto = to@example.com\nsubject = Command result\n[host]\naddress = mail.example.com\nuser = username\npassword = ***\n")
		os.Exit(0)
	}

	config := struct {
		Message struct {
			From, To, Subject string
		}
		Host struct {
			Address, Port, User, Password string
		}
	}{}

	// Read configuration file
	currentUser, _ := user.Current()
	configFileRead := true
	err := gcfg.ReadFileInto(&config, path.Join(currentUser.HomeDir, ".cmdmailer.conf"))
	if err != nil {
		configFileRead = false
	}

	// Set and validate program options
	missingConfiguration := false
	if len(*fromFlag) > 0 {
		config.Message.From = *fromFlag
	}
	if len(config.Message.From) == 0 {
		fmt.Println("Error: No author email address")
		missingConfiguration = true
	}
	if len(*toFlag) > 0 {
		config.Message.To = *toFlag
	}
	if len(config.Message.To) == 0 {
		fmt.Println("Error: No recipient email address")
		missingConfiguration = true
	}
	if len(*subjectFlag) > 0 {
		config.Message.Subject = *subjectFlag
	}
	// Subject is optional
	if len(*addressFlag) > 0 {
		config.Host.Address = *addressFlag
	}
	if len(config.Host.Address) == 0 {
		fmt.Println("Error: No email server address")
		missingConfiguration = true
	}
	if len(config.Host.Port) == 0 {
		config.Host.Port = "25"
	}
	if len(*userFlag) > 0 {
		config.Host.User = *userFlag
	}
	if len(config.Host.User) == 0 {
		fmt.Println("Error: No email server user name")
		missingConfiguration = true
	}
	if len(*passwordFlag) > 0 {
		config.Host.Password = *passwordFlag
	}
	if len(config.Host.Password) == 0 {
		fmt.Println("Error: No email server password")
		missingConfiguration = true
	}
	if missingConfiguration {
		if !configFileRead {
			fmt.Println("Error: The configuration file ~/.cmdmailer.conf could not be read.")
		}
		os.Exit(126)
	}

	// Check command
	command := flag.Args()
	if len(command) == 0 {
		fmt.Println("Error: No command given")
		os.Exit(127)
	}
	if _, err = exec.LookPath(command[0]); err != nil {
		fmt.Println("Error: Command '", command[0], "' not found")
		os.Exit(127)
	}

	// Build command
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	htmlIOContent := bytes.Buffer{}
	if *mailOutputFlag {
		cmd.Stdout = io.MultiWriter(os.Stdout, bufferCSSWriter{&htmlIOContent, "<span class=\"stdout\">", "</span>"})
		cmd.Stderr = io.MultiWriter(os.Stderr, bufferCSSWriter{&htmlIOContent, "<span class=\"stderr\">", "</span>"})
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Run command
	cmd.Run()

	// Check command status
	exitCode := 0
	commandStatus := "succeeded"
	if !cmd.ProcessState.Success() {
		// Get exit reason
		exitCode = 125
		if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			if status.Exited() {
				exitCode = status.ExitStatus()
				commandStatus = "failed with exit code: " + strconv.Itoa(exitCode)
			} else if status.Signaled() {
				commandStatus = "failed with signal: " + html.EscapeString(status.Signal().String())
			} else {
				commandStatus = "failed with an unknown error."
			}
		} else {
			commandStatus = "failed with an unknown error."
		}
	}
	if len(config.Message.Subject) == 0 {
		config.Message.Subject = "Process " + commandStatus
	}

	// Compose email
	ioContent := strings.Replace(htmlIOContent.String(), "\n", "<br>\n", -1)
	content := bytes.Buffer{}
	content.WriteString(`<!doctype html>
<html lang=en>
<head>
<meta charset=utf-8>
<title>`)
	content.WriteString(config.Message.Subject)
	content.WriteString(`</title>
<style>
.stderr{color:#f00;}
</style>
</head>
<body>
<h1>Process '`)
	content.WriteString(html.EscapeString(command[0]))
	content.WriteString("' ")
	content.WriteString(commandStatus)

	content.WriteString(`</h1>
`)
	content.WriteString("Command: ")
	content.WriteString(html.EscapeString(strings.Join(cmd.Args, " ")))
	content.WriteString(`<br>
Execution took: `)
	content.WriteString(cmd.ProcessState.SystemTime().String())
	content.WriteString(" (system) ")
	content.WriteString(cmd.ProcessState.UserTime().String())
	content.WriteString(` (user)<br>
<br>
`)
	if *mailOutputFlag {
		content.WriteString(`STDOUT (black) and STDERR (red) follows:<br>
<hr><br>
`)
		content.WriteString(ioContent)
		if len(ioContent) >= 1000000 {
			content.WriteString(`
<hr>
and more ...`)
		}
	}
	content.WriteString(`
</body>
</html>`)

	// Send email
	message := mail.Message{From: mail.ToAddress(config.Message.From), To: []mail.Address{mail.ToAddress(config.Message.To)}, Subject: config.Message.Subject, Content: mail.MIMEHTML(content.String())}
	mailConfig := mail.SMTPConfig{config.Host.User, config.Host.Password, config.Host.Address, config.Host.Port}
	err = message.Send(mailConfig)
	if err != nil {
		exitCode = 1
	}

	os.Exit(exitCode)
}
