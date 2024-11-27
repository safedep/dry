package async

import "strings"

func RpcTopicName(serviceName, methodName string) string {
	fixer := func(r string) string {
		if len(r) == 0 {
			return ""
		}

		if r[0] == '.' || r[0] == '/' {
			r = r[1:]
		}

		return strings.ReplaceAll(r, "/", ".")
	}

	return fixer(serviceName) + "." + fixer(methodName)
}
