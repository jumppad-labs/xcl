package providers

import (
	"encoding/json"
	"strings"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
)

// previousAttachments returns the network blocks the container had before
// the changes: the current blocks with each network change's previous value
// put back. An element or key added by a change is dropped and one removed by
// a change comes back.
func previousAttachments(current []entities.NetworkAttachment, changes []entity.PropertyChange) ([]entities.NetworkAttachment, error) {
	data, err := json.Marshal(current)
	if err != nil {
		return nil, err
	}

	var networks any
	if err := json.Unmarshal(data, &networks); err != nil {
		return nil, err
	}

	if networks == nil {
		networks = []any{}
	}

	for _, change := range changes {
		if !change.Within(networksSetting) {
			continue
		}

		before := change.Before
		if before == nil {
			before = removed
		}

		networks = putAt(networks, change.Path[len(networksSetting):], before)
	}

	data, err = json.Marshal(dropRemoved(networks))
	if err != nil {
		return nil, err
	}

	previous := []entities.NetworkAttachment{}
	if err := json.Unmarshal(data, &previous); err != nil {
		return nil, err
	}

	return previous, nil
}

// putAt returns value with the element at path set to element, growing a
// list that is too short
func putAt(value any, path entity.Path, element any) any {
	if len(path) == 0 {
		return element
	}

	step := path[0]
	switch step.Kind {
	case entity.StepIndex:
		list, _ := value.([]any)
		for len(list) <= step.Index {
			list = append(list, removed)
		}

		list[step.Index] = putAt(list[step.Index], path[1:], element)
		return list

	default:
		object, _ := value.(map[string]any)
		if object == nil {
			object = map[string]any{}
		}

		key := step.Attribute
		if step.Kind == entity.StepKey {
			key = step.Key
		}

		object[key] = putAt(object[key], path[1:], element)
		return object
	}
}

// dropRemoved returns value without the elements and entries marked removed
func dropRemoved(value any) any {
	switch typed := value.(type) {
	case []any:
		kept := []any{}
		for _, element := range typed {
			if element != removed {
				kept = append(kept, dropRemoved(element))
			}
		}

		return kept

	case map[string]any:
		kept := map[string]any{}
		for key, element := range typed {
			if element != removed {
				kept[key] = dropRemoved(element)
			}
		}

		return kept
	}

	return value
}

// findAttachment returns the network block attaching to the named network
func findAttachment(attachments []entities.NetworkAttachment, name string) (entities.NetworkAttachment, bool) {
	for _, attachment := range attachments {
		if attachment.Name == name {
			return attachment, true
		}
	}

	return entities.NetworkAttachment{}, false
}

// isNetworkAddress returns true when the address is a docker "network"
// block's, such as docker.network.app or module.a.docker.network.app
func isNetworkAddress(address string) bool {
	segments := strings.Split(address, ".")
	if len(segments) < 3 {
		return false
	}

	return segments[len(segments)-3] == "docker" && segments[len(segments)-2] == "network"
}

// networkName returns the Docker name of the network at a docker "network"
// block's address, its block name
func networkName(address string) string {
	return address[strings.LastIndex(address, ".")+1:]
}

// networksSetting is where a container's network blocks are
var networksSetting = entity.Path{}.Attribute("network")

// removed marks a list element or map entry that did not exist before
var removed = &struct{}{}
