package indexer

import "strings"

func isHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".")
}

func isHiddenRelPath(relPath string) bool {
	return strings.HasPrefix(relPath, ".")
}

func isMarkdownFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".md")
}
