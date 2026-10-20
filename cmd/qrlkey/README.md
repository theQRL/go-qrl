qrlkey
======

qrlkey is a simple command-line tool for working with QRL keyfiles.


# Usage

### `qrlkey generate`

Generate a new keyfile.
If you want to use an existing wallet seed in the keyfile, it can be specified
by setting `--seed` with the location of the file containing the wallet seed.


### `qrlkey inspect <keyfile>`

Print various information about the keyfile.
Seed information can be printed by using the `--private` flag;
make sure to use this feature with great caution!


### `qrlkey signmessage <keyfile> <message/file>`

Sign the message with a keyfile.
It is possible to refer to a file containing the message.
To sign a message contained in a file, use the `--msgfile` flag.


### `qrlkey verifymessage <signature> <publickey> <message/file>`

Verify the signature of the message.
It is possible to refer to a file containing the message.
To verify a message contained in a file, use the `--msgfile` flag.


### `qrlkey changepassword <keyfile>`

Change the password of a keyfile.
use the `--newpasswordfile` to point to the new password file.


## Passwords

For every command that uses a keyfile, you will be prompted to provide the 
password for decrypting the keyfile.  To avoid this message, it is possible
to pass the password by using the `--passwordfile` flag pointing to a file that
contains the password.

## JSON

In case you need to output the result in a JSON format, you shall by using the `--json` flag.
