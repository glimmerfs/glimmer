# GlimmerFS Management Server

This is the Glimmer Management Service (MGS) implementation.
This is intended to be a replacement for a Lustre MGS.

Main differences between Lustre MGS and Glimmer MGS:
* Glimmer MGS runs in userland
    * Lustre MGS runs in Kernel space
* Glimmer MGS can even run as non-root if they have permission to bind to 988
* Glimmer MGS does not require a formatted disk to operate on
    * Lustre MGS requires an osd-ldiskfs/osd-zfs formatted MGT (Management target)
* Multiple Glimmer MGSes can run concurrently to increase availability
    * Lustre MGS only supports active/passive failover

## Development

### Adding new manager commands

We use `cobra-cli` to add new commands.

General syntax for a new command (`my-command` in this example):
```
cobra-cli add my-command --config ./cobra.yaml --viper
```
These commands can then be invoked via `manager my-command`
or `go run . my-command` in development.

## License

GlimmerFS Management Server is licensed under GPL v2.0-only.
This code derives some functionality from LustreFS.
