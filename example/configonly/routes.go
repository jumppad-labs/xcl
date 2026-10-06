package main

import (
	"fmt"

	"github.com/jumppad-labs/xcl/example/configonly/resources"
)

// Route is one ingress path and where its traffic ends up: the service the
// ingress sends it to, the deployment that service selects, and the container
// and port inside the deployment that receive it.
type Route struct {
	// Host is the ingress host, i.e. "api.example.com"
	Host string
	// Path is the rule path, i.e. "/"
	Path string
	// Service is the id of the service the rule sends to, i.e. "service.api"
	Service string
	// ServicePort is the port the rule sends to on the service, i.e. 80
	ServicePort int
	// Deployment is the id of the deployment the service selects, i.e.
	// "deployment.api"
	Deployment string
	// Container is the name of the container exposing the service's target
	// port, i.e. "api"
	Container string
	// PortName is the name of that container port, i.e. "http"
	PortName string
	// Port is the container port, i.e. 8080
	Port int
}

// String returns the route as the program prints it, every hop on one line:
//
//	api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)
func (r Route) String() string {
	return fmt.Sprintf(
		"%s%s -> %s:%d -> %s container %s port %s (%d)",
		r.Host, r.Path, r.Service, r.ServicePort, r.Deployment, r.Container, r.PortName, r.Port,
	)
}

// ingressRoutes works out a Route for every rule of every ingress, in the
// order they are declared.
//
// The configuration links the blocks by id: a rule names its service, and a
// service names its deployment, so each hop is a lookup rather than a match on
// labels as Kubernetes would do. The last hop is the service's target port,
// which is found on the first container port, in declaration order, that
// listens on it.
//
// A rule whose service, deployment or target port can not be found is an
// error naming the ingress and path, a route is never silently left out.
func ingressRoutes(cfg *appConfig) ([]Route, error) {
	services := map[string]*resources.Service{}
	for _, service := range cfg.Services {
		services[service.Meta.ID] = service
	}

	deployments := map[string]*resources.Deployment{}
	for _, deployment := range cfg.Deployments {
		deployments[deployment.Meta.ID] = deployment
	}

	routes := []Route{}

	for _, ingress := range cfg.Ingresses {
		for _, rule := range ingress.Rules {
			service, ok := services[rule.Service]
			if !ok {
				return nil, fmt.Errorf("ingress %s path %s: service %q not found", ingress.Meta.ID, rule.Path, rule.Service)
			}

			deployment, ok := deployments[service.Deployment]
			if !ok {
				return nil, fmt.Errorf("ingress %s path %s: deployment %q of service %s not found", ingress.Meta.ID, rule.Path, service.Deployment, service.Meta.ID)
			}

			container, port, ok := findContainerPort(deployment, service.TargetPort)
			if !ok {
				return nil, fmt.Errorf("ingress %s path %s: no container in %s exposes port %d", ingress.Meta.ID, rule.Path, deployment.Meta.ID, service.TargetPort)
			}

			routes = append(routes, Route{
				Host:        ingress.Host,
				Path:        rule.Path,
				Service:     service.Meta.ID,
				ServicePort: rule.Port,
				Deployment:  deployment.Meta.ID,
				Container:   container.Name,
				PortName:    port.Name,
				Port:        port.ContainerPort,
			})
		}
	}

	return routes, nil
}

// findContainerPort returns the first container of deployment, and its port,
// that listens on containerPort. ok is false when no container does.
func findContainerPort(deployment *resources.Deployment, containerPort int) (container resources.Container, port resources.Port, ok bool) {
	for _, container := range deployment.Containers {
		for _, port := range container.Ports {
			if port.ContainerPort == containerPort {
				return container, port, true
			}
		}
	}

	return resources.Container{}, resources.Port{}, false
}
