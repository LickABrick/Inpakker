package tui

type RouteKind int

const (
	RouteApplications RouteKind = iota
	RouteApplication
	RouteDiagnostics
	RouteValidationResults
	RouteBuildResults
	RouteUnpackResults
	RouteLogs
	RouteWorkspaces
	RouteSettings
	RouteAbout
)

type Route struct {
	Kind  RouteKind
	AppID string
}

func (m *Model) currentRoute() Route {
	if len(m.routes) == 0 {
		return Route{Kind: RouteApplications}
	}
	return m.routes[len(m.routes)-1]
}

func (m *Model) pushRoute(route Route) {
	current := m.currentRoute()
	if current.Kind == route.Kind && current.AppID == route.AppID {
		return
	}
	m.routes = append(m.routes, route)
	if route.Kind == RouteApplication {
		m.detailViewport.GotoTop()
	}
}

func (m *Model) popRoute() bool {
	if len(m.routes) <= 1 {
		return false
	}
	m.routes = m.routes[:len(m.routes)-1]
	return true
}
