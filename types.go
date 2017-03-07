package main

import (
	"time"
)

// Zone represents a zone in Cloudflare (https://api.cloudflare.com/#zone-list-zones).
type Zone struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	DevMode           int       `json:"development_mode"`
	OriginalNS        []string  `json:"original_name_servers"`
	OriginalRegistrar string    `json:"original_registrar"`
	OriginalDNSHost   string    `json:"original_dnshost"`
	CreatedOn         time.Time `json:"created_on"`
	ModifiedOn        time.Time `json:"modified_on"`
	NameServers       []string  `json:"name_servers"`
	Permissions       []string  `json:"permissions"`
	Status            string    `json:"status"`
	Paused            bool      `json:"paused"`
	Type              string    `json:"type"`
	Host              struct {
		Name    string
		Website string
	} `json:"host"`
	VanityNS    []string `json:"vanity_name_servers"`
	Betas       []string `json:"betas"`
	DeactReason string   `json:"deactivation_reason"`
}

// DNSRecord represents a dns record in Cloudflare (https://api.cloudflare.com/#dns-records-for-a-zone-list-dns-records).
type DNSRecord struct {
	ID         string      `json:"id,omitempty"`
	Type       string      `json:"type,omitempty"`
	Name       string      `json:"name,omitempty"`
	Content    string      `json:"content,omitempty"`
	Proxiable  bool        `json:"proxiable,omitempty"`
	Proxied    bool        `json:"proxied,omitempty"`
	TTL        int         `json:"ttl,omitempty"`
	Locked     bool        `json:"locked,omitempty"`
	ZoneID     string      `json:"zone_id,omitempty"`
	ZoneName   string      `json:"zone_name,omitempty"`
	CreatedOn  time.Time   `json:"created_on,omitempty"`
	ModifiedOn time.Time   `json:"modified_on,omitempty"`
	Data       interface{} `json:"data,omitempty"` // data returned by: SRV, LOC
	Meta       interface{} `json:"meta,omitempty"`
	Priority   int         `json:"priority,omitempty"`
}

// APIAuthentication contains the email address and api key to authenticate a request to the cloudflare api.
type APIAuthentication struct {
	Key, Email string
}

type dNSRecordsResult struct {
	Success    bool        `json:"success"`
	Errors     interface{} `json:"errors"`
	Messages   interface{} `json:"messages"`
	DNSRecords []DNSRecord `json:"result"`
	ResultInfo resultInfo  `json:"result_info,omitempty"`
}

type zonesResult struct {
	Success    bool        `json:"success"`
	Errors     interface{} `json:"errors"`
	Messages   interface{} `json:"messages"`
	Zones      []Zone      `json:"result"`
	ResultInfo resultInfo  `json:"result_info"`
}

type resultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Count      int `json:"count"`
	TotalCount int `json:"total_count"`
}

type createResult struct {
	Success   bool        `json:"success"`
	Errors    interface{} `json:"errors"`
	Messages  interface{} `json:"messages"`
	DNSRecord DNSRecord   `json:"result,omitempty"`
}

type updateResult struct {
	Success   bool        `json:"success"`
	Errors    interface{} `json:"errors"`
	Messages  interface{} `json:"messages"`
	DNSRecord DNSRecord   `json:"result,omitempty"`
}

type deleteResult struct {
	Success  bool        `json:"success"`
	Errors   interface{} `json:"errors"`
	Messages interface{} `json:"messages"`
	Result   interface{} `json:"result"`
}
