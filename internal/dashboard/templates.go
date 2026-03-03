package dashboard

// View structs used by calendar.go (MonthGridToView) and API schedule responses.

type AgentView struct {
	Name        string
	State       string
	CurrentTask string
	LastActive  string
}

type TaskView struct {
	ID       string
	Name     string
	Status   string
	Duration string
	LastRun  string
}

type IndexView struct {
	SubsystemCount int
	Uptime         string
}

type LoginView struct {
	Error string
	Next  string
}

type AgentsView struct {
	Agents []AgentView
}

type TasksView struct {
	Tasks    []TaskView
	NextPage int
}

type EventView struct {
	Name        string
	Status      string // "ok", "failed", "running", "scheduled"
	TimeStr     string // formatted time string e.g. "14:30"
	IsProjected bool
	IsHighFreq  bool
}

type DayView struct {
	DayNum         int    // 1-31
	DateStr        string // "2006-01-02"
	IsCurrentMonth bool
	IsToday        bool
	Events         []EventView
	HeartbeatOK    *bool // nil = no data, true = healthy, false = unhealthy
}

type WeekView struct {
	Days [7]DayView
}

type ScheduleView struct {
	Year      int
	Month     int    // 1-12
	MonthName string // "January"
	Weeks     []WeekView
	PrevYear  int
	PrevMonth int
	NextYear  int
	NextMonth int
}
