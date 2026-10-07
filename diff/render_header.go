package diff

import (
	"strconv"
	"strings"
)

// blockHeader returns the configuration block header of the resource at
// address, such as resource "container" "api" for resource.container.api or
// server "web" for server.web.
//
// A module prefix such as module.app. is set aside, since the block is
// declared inside the module; the comment line above the header shows the
// full address. Inside a module the block starts at a resource segment when
// there is one, and otherwise is read as its last two segments, or its last
// three when there are three or more after the module name.
func blockHeader(address string) string {
	segments := strings.Split(address, ".")
	body := segments

	if len(segments) > 2 && segments[0] == "module" {
		body = moduleBody(segments)
	}

	switch len(body) {
	case 3:
		return body[0] + " " + strconv.Quote(body[1]) + " " + strconv.Quote(body[2])
	case 2:
		return body[0] + " " + strconv.Quote(body[1])
	case 1:
		return body[0]
	}

	if len(body) > 0 && body[0] != "" {
		return body[0] + " " + strconv.Quote(strings.Join(body[1:], "."))
	}

	return strconv.Quote(address)
}

// moduleBody returns the segments of a module-prefixed address that name the
// block within its module
func moduleBody(segments []string) []string {
	for i := 2; i < len(segments); i++ {
		if segments[i] == "resource" {
			return segments[i:]
		}
	}

	rest := segments[2:]
	if len(rest) >= 3 {
		return rest[len(rest)-3:]
	}

	return rest
}
