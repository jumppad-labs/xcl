package coreservice

import (
	"encoding/json"

	"github.com/jumppad-labs/xcl"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/highlight"
)

// Lookups answer from what the configuration holds between operations. Each
// entity crosses as JSON through encoding/json, so every sensitive value is
// the marker and a host never receives a secret from a lookup.

// entities is Entities.
func (s *Service) entities(conf *configuration, _ *corepb.EntitiesRequest) (*corepb.EntitiesResponse, error) {
	if err := conf.lookupAllowed("Entities"); err != nil {
		return nil, err
	}

	found := conf.cfg.Entities()

	resp := &corepb.EntitiesResponse{Entities: make([][]byte, 0, len(found))}
	for _, e := range found {
		data, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}

		resp.Entities = append(resp.Entities, data)
	}

	return resp, nil
}

// find is Find, an entity by address.
func (s *Service) find(conf *configuration, req *corepb.FindRequest) (*corepb.EntityResponse, error) {
	if err := conf.lookupAllowed("Find"); err != nil {
		return nil, err
	}

	if req.GetAddress() == "" {
		return nil, invalidArgument("an address is required", "argument", "address")
	}

	entity, err := conf.cfg.FindResource(req.GetAddress())
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}

	return &corepb.EntityResponse{Entity: data}, nil
}

// findByType is FindByType, every entity of the type the segments name.
func (s *Service) findByType(conf *configuration, req *corepb.FindByTypeRequest) (*corepb.EntitiesResponse, error) {
	if err := conf.lookupAllowed("FindByType"); err != nil {
		return nil, err
	}

	found, err := conf.cfg.FindEntitiesByType(req.GetPath()...)
	if err != nil {
		return nil, err
	}

	resp := &corepb.EntitiesResponse{Entities: make([][]byte, 0, len(found))}
	for _, e := range found {
		data, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}

		resp.Entities = append(resp.Entities, data)
	}

	return resp, nil
}

// findOne is FindOne, the single entity of the type the segments name.
func (s *Service) findOne(conf *configuration, req *corepb.FindByTypeRequest) (*corepb.EntityResponse, error) {
	if err := conf.lookupAllowed("FindOne"); err != nil {
		return nil, err
	}

	entity, err := conf.cfg.FindOneEntity(req.GetPath()...)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}

	return &corepb.EntityResponse{Entity: data}, nil
}

// outputs is Outputs, the configuration's outputs as one JSON object.
func (s *Service) outputs(conf *configuration, _ *corepb.OutputsRequest) (*corepb.OutputsResponse, error) {
	if err := conf.lookupAllowed("Outputs"); err != nil {
		return nil, err
	}

	values := conf.cfg.Outputs()
	if values == nil {
		values = map[string]any{}
	}

	data, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return &corepb.OutputsResponse{OutputsJson: data}, nil
}

// encodeSavedEntity is EncodeSavedEntity, a saved record rendered as
// configuration text, coloured for a terminal when asked.
func (s *Service) encodeSavedEntity(conf *configuration, req *corepb.EncodeSavedEntityRequest) (*corepb.EncodeSavedEntityResponse, error) {
	if err := conf.lookupAllowed("EncodeSavedEntity"); err != nil {
		return nil, err
	}

	if len(req.GetEntity()) == 0 {
		return nil, invalidArgument("an entity is required", "argument", "entity")
	}

	options := []xcl.EncodeOption{}
	if req.GetHighlight() {
		renderer, err := highlight.NewANSI()
		if err != nil {
			return nil, err
		}

		options = append(options, xcl.Highlight(renderer))
	}

	text, err := conf.cfg.EncodeSavedEntity(req.GetEntity(), options...)
	if err != nil {
		return nil, err
	}

	return &corepb.EncodeSavedEntityResponse{Text: text}, nil
}
