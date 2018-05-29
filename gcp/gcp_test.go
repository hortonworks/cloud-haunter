package gcp

import (
	"fmt"
)

func Example_givenZoneUrl_whenGetRegionFromZoneUrl_thenReturnsRegion() {
	zones := []string{
		"https://www.googleapis.com/compute/v1/projects/projectname/zones/europe-west1-b",
		"https://www.googleapis.com/compute/v1/projects/projectname/zones/us-west1-a",
		"https://www.googleapis.com/compute/v1/projects/projectname/zones/australia-southeast1-a",
	}

	for _, zone := range zones {
		fmt.Println(getRegionFromZoneUrl(&zone))
	}

	// Output:
	// europe-west1
	// us-west1
	// australia-southeast1
}
