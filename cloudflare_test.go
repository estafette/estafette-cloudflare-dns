package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetZoneByDNSName(t *testing.T) {

	t.Run("ReturnsErrorWhenDnsNameIsEmptyString", func(t *testing.T) {

		dnsName := ""
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.GetZoneByDNSName(dnsName)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorWhenDnsNameIsOnlyATLD", func(t *testing.T) {

		dnsName := "com"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.GetZoneByDNSName(dnsName)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsZoneWhenDnsNameEqualsAnExistingZone", func(t *testing.T) {

		dnsName := "server.com"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=server.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "server.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		zone, err := apiClient.GetZoneByDNSName(dnsName)

		assert.Nil(t, err)
		assert.Equal(t, "023e105f4ecef8ad9ca31a8372d0c353", zone.ID)
		assert.Equal(t, "server.com", zone.Name)
	})

}

func TestGetZonesByName(t *testing.T) {

	t.Run("ReturnsEmptyArrayIfNoZoneMatchesName", func(t *testing.T) {

		zoneName := "server.com"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=server.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		zonesResult, err := apiClient.getZonesByName(zoneName)

		assert.Nil(t, err)
		assert.Equal(t, 0, len(zonesResult.Zones))
	})

	t.Run("ReturnsSingleZoneIfZoneMatchesName", func(t *testing.T) {

		zoneName := "server.com"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=server.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "server.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		zonesResult, err := apiClient.getZonesByName(zoneName)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(zonesResult.Zones))
		assert.Equal(t, "023e105f4ecef8ad9ca31a8372d0c353", zonesResult.Zones[0].ID)
		assert.Equal(t, "server.com", zonesResult.Zones[0].Name)
	})

	t.Run("ReturnsMultipleZonesIfMoreThanOneZoneMatchesName", func(t *testing.T) {

		zoneName := "co.uk"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=co.uk", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f48ad9ca31a8372d0c353ecef",
					"name": "domain.co.uk",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				},
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "server.co.uk",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		zonesResult, err := apiClient.getZonesByName(zoneName)

		assert.Nil(t, err)
		assert.Equal(t, 2, len(zonesResult.Zones))
		assert.Equal(t, "023e105f48ad9ca31a8372d0c353ecef", zonesResult.Zones[0].ID)
		assert.Equal(t, "domain.co.uk", zonesResult.Zones[0].Name)
		assert.Equal(t, "023e105f4ecef8ad9ca31a8372d0c353", zonesResult.Zones[1].ID)
		assert.Equal(t, "server.co.uk", zonesResult.Zones[1].Name)
	})
}

func TestGetDNSRecordsByZoneAndName(t *testing.T) {

	t.Run("ReturnsEmptyArrayIfNoDnsRecordsMatchesName", func(t *testing.T) {

		zone := Zone{ID: "023e105f4ecef8ad9ca31a8372d0c353"}
		dnsRecordName := "www"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		dnsRecordsResult, err := apiClient.getDNSRecordsByZoneAndName(zone, dnsRecordName)

		assert.Nil(t, err)
		assert.Equal(t, 0, len(dnsRecordsResult.DNSRecords))
	})

	t.Run("ReturnsDnsRecordIfADnsRecordMatchesName", func(t *testing.T) {

		zone := Zone{ID: "023e105f4ecef8ad9ca31a8372d0c353"}
		dnsRecordName := "www"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		dnsRecordsResult, err := apiClient.getDNSRecordsByZoneAndName(zone, dnsRecordName)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(dnsRecordsResult.DNSRecords))
	})

}

func TestCreateDNSRecord(t *testing.T) {

	t.Run("ReturnsErrorIfZoneDoesNotExist", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "example.com"
		dnsRecordContent := "1.2.3.4"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.CreateDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsRecordIfCreated", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "example.com"
		dnsRecordContent := "1.2.3.4"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		newDNSRecord := DNSRecord{Type: dnsRecordType, Name: dnsRecordName, Content: dnsRecordContent}

		fakeRESTClient.On("Post", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records", newDNSRecord, authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		dnsRecord, err := apiClient.CreateDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.Nil(t, err)
		assert.Equal(t, "372e67954025e0ba6aaa6d586b9e0b59", dnsRecord.ID)
		assert.Equal(t, "A", dnsRecord.Type)
		assert.Equal(t, "example.com", dnsRecord.Name)
		assert.Equal(t, "1.2.3.4", dnsRecord.Content)
		assert.Equal(t, "023e105f4ecef8ad9ca31a8372d0c353", dnsRecord.ZoneID)
	})

}

func TestDeleteDNSRecord(t *testing.T) {

	t.Run("ReturnsErrorIfZoneDoesNotExist", func(t *testing.T) {

		dnsRecordName := "example.com"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.DeleteDNSRecord(dnsRecordName)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorIfDnsRecordDoesNotExist", func(t *testing.T) {

		dnsRecordName := "www.example.com"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.DeleteDNSRecord(dnsRecordName)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorIfDeletingFailed", func(t *testing.T) {

		dnsRecordName := "www.example.com"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		fakeRESTClient.On("Delete", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/372e67954025e0ba6aaa6d586b9e0b59", authentication).Return([]byte(`
			{
				"success": false,
				"errors": [],
				"messages": []
			}
		`), nil)
		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.DeleteDNSRecord(dnsRecordName)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsTrueIfDeletingSucceeded", func(t *testing.T) {

		dnsRecordName := "www.example.com"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		fakeRESTClient.On("Delete", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/372e67954025e0ba6aaa6d586b9e0b59", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"id": "372e67954025e0ba6aaa6d586b9e0b59"
				}
			}
		`), nil)
		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		success, err := apiClient.DeleteDNSRecord(dnsRecordName)

		assert.Nil(t, err)
		assert.True(t, success)
	})

}

func TestUpdateDNSRecord(t *testing.T) {

	t.Run("ReturnsErrorIfZoneDoesNotExist", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "example.com"
		dnsRecordContent := "1.2.3.4"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.UpdateDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorIfDnsRecordDoesNotExist", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.4"

		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.UpdateDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorIfTypeIsDifferent", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.4"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "CNAME",
					"name": "www.example.com",
					"content": "example.com",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.UpdateDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorIfUpdateFailed", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.5"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		createdOn, err := time.Parse("2006-01-02T15:04:05.00000Z", "2014-01-01T05:20:00.12345Z")
		modifiedOn, err := time.Parse("2006-01-02T15:04:05.00000Z", "2014-01-01T05:20:00.12345Z")

		updatedDNSRecord := DNSRecord{
			ID:         "372e67954025e0ba6aaa6d586b9e0b59",
			Type:       dnsRecordType,
			Name:       dnsRecordName,
			Content:    dnsRecordContent,
			Proxiable:  true,
			Proxied:    false,
			TTL:        120,
			Locked:     false,
			ZoneID:     "023e105f4ecef8ad9ca31a8372d0c353",
			ZoneName:   "example.com",
			CreatedOn:  createdOn,
			ModifiedOn: modifiedOn,
			Data:       map[string]interface{}{},
		}

		fakeRESTClient.On("Put", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/372e67954025e0ba6aaa6d586b9e0b59", updatedDNSRecord, authentication).Return([]byte(`
			{
				"success": false,
				"errors": [],
				"messages": [],
				"result": {}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err = apiClient.UpdateDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsUpdatedDnsRecordIfUpdateSucceeded", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.5"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		createdOn, err := time.Parse("2006-01-02T15:04:05.00000Z", "2014-01-01T05:20:00.12345Z")
		modifiedOn, err := time.Parse("2006-01-02T15:04:05.00000Z", "2014-01-01T05:20:00.12345Z")

		updatedDNSRecord := DNSRecord{
			ID:         "372e67954025e0ba6aaa6d586b9e0b59",
			Type:       dnsRecordType,
			Name:       dnsRecordName,
			Content:    dnsRecordContent,
			Proxiable:  true,
			Proxied:    false,
			TTL:        120,
			Locked:     false,
			ZoneID:     "023e105f4ecef8ad9ca31a8372d0c353",
			ZoneName:   "example.com",
			CreatedOn:  createdOn,
			ModifiedOn: modifiedOn,
			Data:       map[string]interface{}{},
		}

		fakeRESTClient.On("Put", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/372e67954025e0ba6aaa6d586b9e0b59", updatedDNSRecord, authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.5",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2016-01-01T05:20:00.12345Z",
					"data": {}
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		returnedDNSRecord, err := apiClient.UpdateDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.Nil(t, err)
		assert.Equal(t, "1.2.3.5", returnedDNSRecord.Content)
	})

}

func TestUpsertDNSRecord(t *testing.T) {

	t.Run("ReturnsErrorIfZoneDoesNotExist", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "example.com"
		dnsRecordContent := "1.2.3.4"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.UpsertDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorIfDnsRecordDoesNotExistAndCreateFails", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.4"

		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		newDNSRecord := DNSRecord{Type: dnsRecordType, Name: dnsRecordName, Content: dnsRecordContent}

		fakeRESTClient.On("Post", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records", newDNSRecord, authentication).Return([]byte(`
			{
				"success": false,
				"errors": [],
				"messages": [],
				"result": {}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.UpsertDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsDnsRecordIfDnsRecordDoesNotExistAndCreateSucceeds", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.4"

		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 0,
					"total_count": 0
				}
			}
		`), nil)

		newDNSRecord := DNSRecord{Type: dnsRecordType, Name: dnsRecordName, Content: dnsRecordContent}

		fakeRESTClient.On("Post", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records", newDNSRecord, authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		createdDNSRecord, err := apiClient.UpsertDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.Nil(t, err)
		assert.Equal(t, "372e67954025e0ba6aaa6d586b9e0b59", createdDNSRecord.ID)
	})

	t.Run("ReturnsErrorIfDnsRecordExistsAndTypeIsDifferent", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.4"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "CNAME",
					"name": "www.example.com",
					"content": "example.com",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err := apiClient.UpsertDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsErrorIfDnsRecordExistsAndUpdateFailed", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.5"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		createdOn, err := time.Parse("2006-01-02T15:04:05.00000Z", "2014-01-01T05:20:00.12345Z")
		modifiedOn, err := time.Parse("2006-01-02T15:04:05.00000Z", "2014-01-01T05:20:00.12345Z")

		updatedDNSRecord := DNSRecord{
			ID:         "372e67954025e0ba6aaa6d586b9e0b59",
			Type:       dnsRecordType,
			Name:       dnsRecordName,
			Content:    dnsRecordContent,
			Proxiable:  true,
			Proxied:    false,
			TTL:        120,
			Locked:     false,
			ZoneID:     "023e105f4ecef8ad9ca31a8372d0c353",
			ZoneName:   "example.com",
			CreatedOn:  createdOn,
			ModifiedOn: modifiedOn,
			Data:       map[string]interface{}{},
		}

		fakeRESTClient.On("Put", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/372e67954025e0ba6aaa6d586b9e0b59", updatedDNSRecord, authentication).Return([]byte(`
			{
				"success": false,
				"errors": [],
				"messages": [],
				"result": {}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		_, err = apiClient.UpsertDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.NotNil(t, err)
	})

	t.Run("ReturnsUpdatedDnsRecordIfDnsRecordExistsAndUpdateSucceeded", func(t *testing.T) {

		dnsRecordType := "A"
		dnsRecordName := "www.example.com"
		dnsRecordContent := "1.2.3.5"
		authentication := APIAuthentication{Key: "r2kjepva04hijzv18u3e9ntphs79kctdxxj5w", Email: "name@server.com"}

		fakeRESTClient := new(fakeRESTClient)
		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/?name=example.com", authentication).Return([]byte(`
		{
			"success": true,
			"errors": [],
			"messages": [],
			"result": [
				{
					"id": "023e105f4ecef8ad9ca31a8372d0c353",
					"name": "example.com",
					"development_mode": 7200,
					"original_name_servers": [
						"ns1.originaldnshost.com",
						"ns2.originaldnshost.com"
					],
					"original_registrar": "GoDaddy",
					"original_dnshost": "NameCheap",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"name_servers": [
						"tony.ns.cloudflare.com",
						"woz.ns.cloudflare.com"
					],
					"owner": {
						"id": "7c5dae5552338874e5053f2534d2767a",
						"email": "user@example.com",
						"owner_type": "user"
					},
					"permissions": [
						"#zone:read",
						"#zone:edit"
					],
					"plan": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"plan_pending": {
						"id": "e592fd9519420ba7405e1307bff33214",
						"name": "Pro Plan",
						"price": 20,
						"currency": "USD",
						"frequency": "monthly",
						"legacy_id": "pro",
						"is_subscribed": true,
						"can_subscribe": true
					},
					"status": "active",
					"paused": false,
					"type": "full",
					"checked_on": "2014-01-01T05:20:00.12345Z"
				}
			],
			"result_info": {
				"page": 1,
				"per_page": 20,
				"count": 1,
				"total_count": 1
			}
		}
		`), nil)

		fakeRESTClient.On("Get", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/?name=www.example.com", authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.4",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2014-01-01T05:20:00.12345Z",
					"data": {}
					}
				],
				"result_info": {
					"page": 1,
					"per_page": 20,
					"count": 1,
					"total_count": 1
				}
			}
		`), nil)

		createdOn, err := time.Parse("2006-01-02T15:04:05.00000Z", "2014-01-01T05:20:00.12345Z")
		modifiedOn, err := time.Parse("2006-01-02T15:04:05.00000Z", "2014-01-01T05:20:00.12345Z")

		updatedDNSRecord := DNSRecord{
			ID:         "372e67954025e0ba6aaa6d586b9e0b59",
			Type:       dnsRecordType,
			Name:       dnsRecordName,
			Content:    dnsRecordContent,
			Proxiable:  true,
			Proxied:    false,
			TTL:        120,
			Locked:     false,
			ZoneID:     "023e105f4ecef8ad9ca31a8372d0c353",
			ZoneName:   "example.com",
			CreatedOn:  createdOn,
			ModifiedOn: modifiedOn,
			Data:       map[string]interface{}{},
		}

		fakeRESTClient.On("Put", "https://api.cloudflare.com/client/v4/zones/023e105f4ecef8ad9ca31a8372d0c353/dns_records/372e67954025e0ba6aaa6d586b9e0b59", updatedDNSRecord, authentication).Return([]byte(`
			{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"id": "372e67954025e0ba6aaa6d586b9e0b59",
					"type": "A",
					"name": "www.example.com",
					"content": "1.2.3.5",
					"proxiable": true,
					"proxied": false,
					"ttl": 120,
					"locked": false,
					"zone_id": "023e105f4ecef8ad9ca31a8372d0c353",
					"zone_name": "example.com",
					"created_on": "2014-01-01T05:20:00.12345Z",
					"modified_on": "2016-01-01T05:20:00.12345Z",
					"data": {}
				}
			}
		`), nil)

		apiClient := New(authentication)
		apiClient.restClient = fakeRESTClient

		// act
		returnedDNSRecord, err := apiClient.UpsertDNSRecord(dnsRecordType, dnsRecordName, dnsRecordContent)

		assert.Nil(t, err)
		assert.Equal(t, "1.2.3.5", returnedDNSRecord.Content)
	})

}
