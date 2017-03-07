package main

import (
	"errors"
	"fmt"
)

func getLastItemsFromSlice(source []string, numberOfItems int) (r []string, err error) {

	if len(source) == 0 {
		err = errors.New("cloudflare: argument source is nil")
		return
	}
	if numberOfItems > len(source) {
		err = fmt.Errorf("cloudflare: argument numberOfItems (%v) is larger than number of items in argument source (%v)", numberOfItems, len(source))
		return
	}

	r = make([]string, numberOfItems)
	sourceLength := len(source)
	r = source[sourceLength-numberOfItems:]
	return
}

func getMatchingZoneFromZones(zones []Zone, zoneName string) (r Zone, err error) {

	if len(zones) == 0 {
		err = errors.New("cloudflare: zones cannot be empty")
		return
	}

	for _, zone := range zones {
		if zone.Name == zoneName {
			r = zone
			return
		}
	}

	err = errors.New("cloudflare: no zone matches name")
	return
}
