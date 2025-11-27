package izanami

import "encoding/json"

// Mapper transforms raw JSON bytes into a typed result.
type Mapper[T any] func([]byte) (T, error)

// Identity returns the raw bytes unchanged.
func Identity(data []byte) ([]byte, error) { return data, nil }

// Unmarshal builds a mapper for values and slices (e.g., []Tenant, []Project).
func Unmarshal[T any]() Mapper[T] {
	return func(data []byte) (T, error) {
		var out T
		err := json.Unmarshal(data, &out)
		return out, err
	}
}

// UnmarshalPtr builds a mapper for single objects returned as pointers (e.g., *Tenant).
func UnmarshalPtr[T any]() Mapper[*T] {
	return func(data []byte) (*T, error) {
		var out T
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return &out, nil
	}
}

var (
	ParseTenants       = Unmarshal[[]Tenant]()
	ParseTenant        = UnmarshalPtr[Tenant]()
	ParseProjects      = Unmarshal[[]Project]()
	ParseProject       = UnmarshalPtr[Project]()
	ParseTags          = Unmarshal[[]Tag]()
	ParseTag           = UnmarshalPtr[Tag]()
	ParseFeatures      = Unmarshal[[]Feature]()
	ParseFeature       = UnmarshalPtr[FeatureWithOverloads]()
	ParseAPIKeys       = Unmarshal[[]APIKey]()
	ParseAPIKey        = UnmarshalPtr[APIKey]()
	ParseUsers         = Unmarshal[[]User]()
	ParseUser          = UnmarshalPtr[User]()
	ParseUserListItems = Unmarshal[[]UserListItem]()
	ParseContexts      = Unmarshal[[]Context]()
	ParseAuditEvents   = Unmarshal[[]AuditEvent]()
	ParseLogsResponse  = UnmarshalPtr[LogsResponse]()
)
