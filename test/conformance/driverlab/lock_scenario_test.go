package driverlab

func (s *scenario) runDriverLockWait() {
	s.t.Helper()
	switch s.provider {
	case "postgres":
		s.checkPostgresLockWait()
	case "sqlite":
		s.checkSQLiteLockWait()
	case "sqlserver":
		s.checkSQLServerLockWait()
	case "mysql":
		s.checkMySQLLockWait()
	default:
		s.t.Fatal("no lock wait fixture for the selected provider")
	}
}

func (s *scenario) checkDriverLockWait() {
	s.t.Helper()
	nativeCodes := map[string]string{"postgres": "55P03", "sqlite": "5", "sqlserver": "1222", "mysql": "1205/HY000"}
	if s.facts["lock_wait_observed"] != "both" ||
		s.facts["lock_deadline_cause"] != "context.DeadlineExceeded" ||
		s.facts["lock_timeout_native"] != nativeCodes[s.provider] {
		s.t.Fatal("missing observed waits or unexpected lock timeout outcomes")
	}
}

func (s *scenario) checkDriverLockRows() {
	s.t.Helper()
	if s.provider == "mysql" {
		s.checkParents("first", "second")
		return
	}
	s.checkParent()
}
