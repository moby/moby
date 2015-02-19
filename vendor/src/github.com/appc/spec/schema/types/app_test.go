package types

import "testing"

func TestAppValid(t *testing.T) {
	tests := []App{
		App{
			Exec:             []string{"/bin/httpd"},
			User:             "0",
			Group:            "0",
			WorkingDirectory: "/tmp",
		},
		App{
			Exec:  []string{"/app"},
			User:  "0",
			Group: "0",
			EventHandlers: []EventHandler{
				{Name: "pre-start"},
				{Name: "post-stop"},
			},
			Environment: []EnvironmentVariable{
				{Name: "DEBUG", Value: "true"},
			},
			WorkingDirectory: "/tmp",
		},
		App{
			Exec:             []string{"/app", "arg1", "arg2"},
			User:             "0",
			Group:            "0",
			WorkingDirectory: "/tmp",
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err != nil {
			t.Errorf("#%d: err == %v, want nil", i, err)
		}
	}
}

func TestAppExecInvalid(t *testing.T) {
	tests := []App{
		App{
			Exec: nil,
		},
		App{
			Exec:  []string{},
			User:  "0",
			Group: "0",
		},
		App{
			Exec:  []string{"app"},
			User:  "0",
			Group: "0",
		},
		App{
			Exec:  []string{"bin/app", "arg1"},
			User:  "0",
			Group: "0",
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err == nil {
			t.Errorf("#%d: err == nil, want non-nil", i)
		}
	}
}

func TestAppEventHandlersInvalid(t *testing.T) {
	tests := []App{
		App{
			Exec:  []string{"/bin/httpd"},
			User:  "0",
			Group: "0",
			EventHandlers: []EventHandler{
				EventHandler{
					Name: "pre-start",
				},
				EventHandler{
					Name: "pre-start",
				},
			},
		},
		App{
			Exec:  []string{"/bin/httpd"},
			User:  "0",
			Group: "0",
			EventHandlers: []EventHandler{
				EventHandler{
					Name: "post-stop",
				},
				EventHandler{
					Name: "pre-start",
				},
				EventHandler{
					Name: "post-stop",
				},
			},
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err == nil {
			t.Errorf("#%d: err == nil, want non-nil", i)
		}
	}
}

func TestUserGroupInvalid(t *testing.T) {
	tests := []App{
		App{
			Exec: []string{"/app"},
		},
		App{
			Exec: []string{"/app"},
			User: "0",
		},
		App{
			Exec:  []string{"app"},
			Group: "0",
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err == nil {
			t.Errorf("#%d: err == nil, want non-nil", i)
		}
	}
}

func TestAppWorkingDirectoryInvalid(t *testing.T) {
	tests := []App{
		App{
			WorkingDirectory: "stuff",
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err == nil {
			t.Errorf("#%d: err == nil, want non-nil", i)
		}
	}
}

func TestAppEnvironmentInvalid(t *testing.T) {
	tests := []App{
		App{
			Environment: Environment{
				EnvironmentVariable{"0DEBUG", "true"},
			},
		},
	}
	for i, tt := range tests {
		if err := tt.assertValid(); err == nil {
			t.Errorf("#%d: err == nil, want non-nil", i)
		}
	}
}
