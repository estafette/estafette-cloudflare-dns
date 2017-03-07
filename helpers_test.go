package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeRESTClient struct {
	mock.Mock
}

func (r *fakeRESTClient) Get(cloudflareAPIURL string, authentication APIAuthentication) (body []byte, err error) {
	args := r.Called(cloudflareAPIURL, authentication)
	return args.Get(0).([]byte), args.Error(1)
}

func (r *fakeRESTClient) Post(cloudflareAPIURL string, params interface{}, authentication APIAuthentication) (body []byte, err error) {
	args := r.Called(cloudflareAPIURL, params, authentication)
	return args.Get(0).([]byte), args.Error(1)
}

func (r *fakeRESTClient) Put(cloudflareAPIURL string, params interface{}, authentication APIAuthentication) (body []byte, err error) {
	args := r.Called(cloudflareAPIURL, params, authentication)
	return args.Get(0).([]byte), args.Error(1)
}

func (r *fakeRESTClient) Delete(cloudflareAPIURL string, authentication APIAuthentication) (body []byte, err error) {
	args := r.Called(cloudflareAPIURL, authentication)
	return args.Get(0).([]byte), args.Error(1)
}

func testEq(a, b []string) bool {

	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestGetLastItemsFromSlice(t *testing.T) {

	t.Run("Returns2ItemsWhenSourceIsLongerAndNrOfItemsIs2", func(t *testing.T) {

		source := []string{"www", "server", "com"}

		// act
		items, err := getLastItemsFromSlice(source, 2)

		assert.Nil(t, err)
		assert.Equal(t, 2, len(items))
		assert.True(t, testEq(items, []string{"server", "com"}))
	})

	t.Run("ReturnsAllItemsWhenNrOfItemsEqualsSourceLength", func(t *testing.T) {

		source := []string{"www", "server", "com"}

		// act
		items, err := getLastItemsFromSlice(source, 3)

		assert.Nil(t, err)
		assert.Equal(t, 3, len(items))
		assert.True(t, testEq(items, []string{"www", "server", "com"}))
	})

	t.Run("ReturnsErrorWhenSourceIsNil", func(t *testing.T) {

		// act
		_, err := getLastItemsFromSlice(nil, 4)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorWhenNrOfItemsIsLargerThanNrOfSourceItems", func(t *testing.T) {

		source := []string{"www", "server", "com"}

		// act
		_, err := getLastItemsFromSlice(source, 4)

		assert.NotNil(t, err)
	})

}

func TestGetMatchingZoneFromZones(t *testing.T) {

	t.Run("NoZonesReturnsError", func(t *testing.T) {

		zones := []Zone{}
		zoneName := "server.com"

		// act
		_, err := getMatchingZoneFromZones(zones, zoneName)

		assert.NotNil(t, err)
	})

	t.Run("NoMatchingZoneReturnsError", func(t *testing.T) {

		zones := []Zone{Zone{ID: "abcd", Name: "server.com"}}
		zoneName := "domain.com"

		// act
		_, err := getMatchingZoneFromZones(zones, zoneName)

		assert.NotNil(t, err)
	})

	t.Run("MatchingZoneReturnsMatchingZone", func(t *testing.T) {

		zones := []Zone{Zone{ID: "abcd", Name: "server.com"}, Zone{ID: "efgh", Name: "domain.com"}}
		zoneName := "domain.com"

		// act
		zone, _ := getMatchingZoneFromZones(zones, zoneName)

		assert.Equal(t, "efgh", zone.ID)
		assert.Equal(t, "domain.com", zone.Name)
	})
}
