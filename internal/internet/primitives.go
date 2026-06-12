package internet

type PrimitiveName string

const (
	PrimitiveInternetSearch PrimitiveName = "internet_search"
	PrimitiveInternetFetch  PrimitiveName = "internet_fetch"
	PrimitiveInternetHead   PrimitiveName = "internet_head"
	PrimitiveCrawlerTask    PrimitiveName = "crawler_task"
)

type PrimitiveResponsibility struct {
	Name           PrimitiveName `json:"name"`
	Responsibility string        `json:"responsibility"`
	NetworkScope   string        `json:"networkScope"`
	Implemented    bool          `json:"implemented"`
}

func PrimitiveResponsibilities() []PrimitiveResponsibility {
	return []PrimitiveResponsibility{
		{
			Name:           PrimitiveInternetSearch,
			Responsibility: "find_urls",
			NetworkScope:   "configured search provider endpoint",
			Implemented:    true,
		},
		{
			Name:           PrimitiveInternetFetch,
			Responsibility: "fetch_one_url",
			NetworkScope:   "single approved URL",
			Implemented:    true,
		},
		{
			Name:           PrimitiveInternetHead,
			Responsibility: "check_one_url_headers",
			NetworkScope:   "single approved URL",
			Implemented:    true,
		},
		{
			Name:           PrimitiveCrawlerTask,
			Responsibility: "follow_many_urls",
			NetworkScope:   "approved bounded same-domain crawl",
			Implemented:    true,
		},
	}
}
