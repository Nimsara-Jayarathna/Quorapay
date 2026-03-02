package coordination

import "strings"

func (m *Manager) membersPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/members"
}

func (m *Manager) memberPath() string {
	return m.membersPath() + "/" + m.cfg.NodeID
}

func (m *Manager) electionPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/election"
}

func (m *Manager) leaderPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/leader"
}

func (m *Manager) termPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/term"
}

func (m *Manager) logHeadPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/log_head"
}
