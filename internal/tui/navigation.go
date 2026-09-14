package tui

type RouteKind int

const (
	RouteApplications RouteKind = iota
	RouteApplication
	RouteDiagnostics
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
	switch route.Kind {
	case RouteApplications, RouteWorkspaces, RouteSettings, RouteAbout:
		m.routes = []Route{route}
	default:
		m.routes = append(m.routes, route)
	}
	if route.Kind == RouteApplication || route.Kind == RouteAbout {
		m.detailViewport.GotoTop()
	}
}

func (m *Model) popRoute() bool {
	if len(m.routes) <= 1 {
		if m.currentRoute().Kind != RouteApplications {
			m.routes = []Route{{Kind: RouteApplications}}
			return true
		}
		return false
	}
	m.routes = m.routes[:len(m.routes)-1]
	return true
}

func (m Model) routeTitle() string {
	switch m.currentRoute().Kind {
	case RouteWorkspaces:
		return "Workspaces"
	case RouteSettings:
		return "Settings"
	case RouteAbout:
		return "About"
	case RouteApplication:
		return "Application"
	case RouteDiagnostics:
		return "Workspace diagnostics"
	}
	return "Applications"
}
