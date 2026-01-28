package internal

import (
	"regexp"

	"github.com/emirpasic/gods/sets/treeset"
)

func FilterByRegex(input []string, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, s := range input {
		if re.MatchString(s) {
			result = append(result, s)
		}
	}
	return result, nil
}

func FilterTreeSetRegex(
	set *treeset.Set,
	pattern string,
) ([]string, error) {

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	result := []string{}
	it := set.Iterator()
	for it.Next() {
		val := it.Value().(string)
		if re.MatchString(val) {
			result = append(result, val)
		}
	}

	return result, nil
}
