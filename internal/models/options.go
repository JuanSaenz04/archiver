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
	Name      string    `json:"name"`
	ScopeType ScopeType `json:"scopeType"`
}
