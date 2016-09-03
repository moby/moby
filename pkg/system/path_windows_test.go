// +build windows

package system

import "github.com/go-check/check"

// TestCheckSystemDriveAndRemoveDriveLetter tests CheckSystemDriveAndRemoveDriveLetter
func (s *DockerSuite) TestCheckSystemDriveAndRemoveDriveLetter(c *check.C) {
	// Fails if not C drive.
	path, err := CheckSystemDriveAndRemoveDriveLetter(`d:\`)
	if err == nil || (err != nil && err.Error() != "The specified path is not on the system drive (C:)") {
		c.Fatalf("Expected error for d:")
	}

	// Single character is unchanged
	if path, err = CheckSystemDriveAndRemoveDriveLetter("z"); err != nil {
		c.Fatalf("Single character should pass")
	}
	if path != "z" {
		c.Fatalf("Single character should be unchanged")
	}

	// Two characters without colon is unchanged
	if path, err = CheckSystemDriveAndRemoveDriveLetter("AB"); err != nil {
		c.Fatalf("2 characters without colon should pass")
	}
	if path != "AB" {
		c.Fatalf("2 characters without colon should be unchanged")
	}

	// Abs path without drive letter
	if path, err = CheckSystemDriveAndRemoveDriveLetter(`\l`); err != nil {
		c.Fatalf("abs path no drive letter should pass")
	}
	if path != `\l` {
		c.Fatalf("abs path without drive letter should be unchanged")
	}

	// Abs path without drive letter, linux style
	if path, err = CheckSystemDriveAndRemoveDriveLetter(`/l`); err != nil {
		c.Fatalf("abs path no drive letter linux style should pass")
	}
	if path != `\l` {
		c.Fatalf("abs path without drive letter linux failed %s", path)
	}

	// Drive-colon should be stripped
	if path, err = CheckSystemDriveAndRemoveDriveLetter(`c:\`); err != nil {
		c.Fatalf("An absolute path should pass")
	}
	if path != `\` {
		c.Fatalf(`An absolute path should have been shortened to \ %s`, path)
	}

	// Verify with a linux-style path
	if path, err = CheckSystemDriveAndRemoveDriveLetter(`c:/`); err != nil {
		c.Fatalf("An absolute path should pass")
	}
	if path != `\` {
		c.Fatalf(`A linux style absolute path should have been shortened to \ %s`, path)
	}

	// Failure on c:
	if path, err = CheckSystemDriveAndRemoveDriveLetter(`c:`); err == nil {
		c.Fatalf("c: should fail")
	}
	if err.Error() != `No relative path specified in "c:"` {
		c.Fatalf(path, err)
	}

	// Failure on d:
	if path, err = CheckSystemDriveAndRemoveDriveLetter(`d:`); err == nil {
		c.Fatalf("c: should fail")
	}
	if err.Error() != `No relative path specified in "d:"` {
		c.Fatalf(path, err)
	}
}
