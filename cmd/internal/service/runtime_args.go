package service

import "fmt"

func RuntimeWorkspaceFromArgs(args []string) (string, bool, error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == InternalRunFlag {
			if i+1 >= len(args) {
				return "", false, fmt.Errorf("service: missing workspace for %s", InternalRunFlag)
			}
			return args[i+1], true, nil
		}
	}
	return "", false, nil
}
