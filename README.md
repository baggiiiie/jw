# Jenkins Watcher (jw)

A daemon that monitors Jenkins jobs in the background and sends macOS notifications upon completion.

## Build

To build the `jw` binary and install it to `$HOME/.local/bin`, run the following command from the root of the repository:

```bash
make build-jw
```

Then make sure `$HOME/.local/bin` is in your `PATH`.
