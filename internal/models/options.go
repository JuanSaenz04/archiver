package models

type ScopeType string

const (
	Page    ScopeType = "page"
	PageSpa ScopeType = "page-spa"
	Prefix  ScopeType = "prefix"
	Host    ScopeType = "host"
	Domain  ScopeType = "domain"
	Any     ScopeType = "any"
)

type CrawlOptions struct {
	ScopeType ScopeType `json:"scopeType"`
	PageLimit int       `json:"page_limit"`
	SizeLimit int       `json:"size_limit"`
	Depth     int       `json:"depth"`
}
