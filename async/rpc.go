package async

import "strings"

func RpcTopicName(serviceName, methodName string) string {
	fixer := func(r string) string {
		if len(r) == 0 {
			return ""
		}

		r = strings.TrimPrefix(r, "/")
		r = strings.TrimSuffix(r, "/")

		return strings.ReplaceAll(r, "/", ".")
	}

	return fixer(serviceName) + "." + fixer(methodName)
}

func RpcTopicNameFromFullProcedureName(fullProcedureName string) string {
	if len(fullProcedureName) == 0 {
		return ""
	}

	if fullProcedureName[0] == '/' {
		fullProcedureName = fullProcedureName[1:]
	}

	parts := strings.SplitN(fullProcedureName, "/", 2)
	if len(parts) < 2 {
		return ""
	}

	return RpcTopicName(parts[0], parts[1])
}
