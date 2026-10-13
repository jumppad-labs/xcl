---
tags: [plugins, go-plugin, stdio, subprocess]
---

# go-plugin hands the host's stdin to every plugin

`github.com/hashicorp/go-plugin` v1.6.3 sets `cmd.Stdin = os.Stdin` for every external plugin process it starts (`client.go`, in `Client.Start`). A plugin therefore inherits whatever the host process's stdin is.

This bites any xcl process that uses its own stdin for a protocol, such as the `cmd/xcl-core` engine program serving length-prefixed frames over stdio. If plugins start while `os.Stdin` is still the protocol pipe, every plugin inherits that pipe and can read from or hold open the host's request stream.

Before any plugin starts, keep the real stdin and stdout for the protocol, then point `os.Stdin` at the null device and `os.Stdout` at stderr. Anything that later writes to stdout, including library logging, then cannot corrupt the frame stream either.
