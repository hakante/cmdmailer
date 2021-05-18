# Cmd status email utility

This is a small utility program for sending command line status emails.

## Example usage

In this source directory, calling

    $ cmdmailer ls

would produce an email similar to

    Process 'ls' succeeded
    Command: ls
    Execution took: 2.854ms (system) 1.743ms (user)

    STDOUT (black) and STDERR (red) follows:

    LICENSE
    README.md
    cmdmailer
    go.mod
    go.sum
    main.go


## Command line flags

This utility has the following command line flags:

      -from string
          Author email address
      -help
          Print this help message and exit
      -host string
          Email server address
      -mail-output
          Include STDOUT/STDERR in email (default true)
      -password string
          Email server password
      -subject string
          Email subject (optional)
      -to string
          Recipient email address
      -user string
          Email server user name


## Configuration file

This utility can also be configured by creating a configuration file `~/.cmdmailer.conf` in your user home directory. The following options are supported:

    [message]
    from = CmdLine <cmdline@example.com>
    to = user@example.com
    subject = Your command has finished
    [host]
    address = smtp.example.com
    port = 587
    user = cmdline@example.com
    password = my_secret_password
