package async

import "fmt"

// DomainEventTopicName returns the topic name for a domain event published by a service.
// This follows a convention-driven approach where a service publishes its own domain events
// to a well-known topic name derived from the service name and event name.
func DomainEventTopicName(serviceName, eventName string) string {
	return fmt.Sprintf("events.%s", RpcTopicName(serviceName, eventName))
}

// DomainEventNamespacedTopicName returns the topic name for a namespaced domain event.
// If the namespace is empty, the behaviour is exactly same as DomainEventTopicName.
func DomainEventNamespacedTopicName(serviceName, eventName, namespace string) string {
	topicName := DomainEventTopicName(serviceName, eventName)
	if len(namespace) == 0 {
		return topicName
	}

	return fmt.Sprintf("namespaced.%s.%s", namespace, topicName)
}

// DomainEventTopicNameFromFullProcedureName generates a domain event topic name from a
// full procedure name (e.g. /safedep.services.foo.v1.FooService/OrderCreated).
// This reuses the same procedure name parsing as RPC but produces an event topic.
func DomainEventTopicNameFromFullProcedureName(fullProcedureName string) string {
	serviceName, eventName, err := rpcGetServiceAndMethodFromFullProcedureName(fullProcedureName)
	if err != nil {
		return ""
	}

	return DomainEventTopicName(serviceName, eventName)
}
