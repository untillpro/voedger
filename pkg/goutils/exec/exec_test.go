/*
 * Copyright (c) 2023-present unTill Software Development Group B. V.  and Contributors
 * @author Maxim Geraskin
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package exec_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/goutils/exec"
)

func Test_PassEnvironmentVariable(t *testing.T) {

	// set MYVAR=MYVALUE
	os.Setenv("MYVAR", "MYVALUE")

	stdout, stderr, err := new(exec.PipedExec).
		Command("sh", "-c", "echo $MYVAR").
		Command("sh", "-c", "grep $MYVAR").
		RunToStrings()
	assert.Nil(t, err)
	assert.Equal(t, "MYVALUE", strings.TrimSpace(stdout))
	assert.Equal(t, "", strings.TrimSpace(stderr))
}

func Test_Wd(t *testing.T) {

	require := require.New(t)

	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	require.NoError(os.WriteFile(filepath.Join(tmpDir1, "1.txt"), []byte("11.txt"), 0644))
	require.NoError(os.WriteFile(filepath.Join(tmpDir2, "2.txt"), []byte("21.txt"), 0644))

	// Run ls commands

	var err error

	err = new(exec.PipedExec).
		Command("ls").WorkingDir(tmpDir1).
		Run(os.Stdout, os.Stdout)
	assert.Nil(t, err)

	err = new(exec.PipedExec).
		Command("ls", "1.txt").WorkingDir(tmpDir1).
		Run(os.Stdout, os.Stdout)
	assert.Nil(t, err)

	err = new(exec.PipedExec).
		Command("ls", "2.txt").WorkingDir(tmpDir2).
		Run(os.Stdout, os.Stdout)
	assert.Nil(t, err)

	err = new(exec.PipedExec).
		Command("ls", "1.txt").WorkingDir(tmpDir2).
		Run(os.Stdout, os.Stdout)
	assert.NotNil(t, err)

	err = new(exec.PipedExec).
		Command("ls", "2.txt").WorkingDir(tmpDir1).
		Run(os.Stdout, os.Stdout)
	assert.NotNil(t, err)

}

func Test_PipeFall(t *testing.T) {

	// echo hi | grep hi | echo good => OK
	{
		err := new(exec.PipedExec).
			Command("echo", "hi").
			Command("grep", "hi").
			Command("echo", "good").
			Run(os.Stdout, os.Stdout)
		require.NoError(t, err)
	}

	// echo hi | grep hello | echo good => FAIL
	{
		err := new(exec.PipedExec).
			Command("echo", "hi").
			Command("grep", "hello").
			Command("echo", "good").
			Run(os.Stdout, os.Stdout)
		require.Error(t, err)
	}
}

func Test_WrongCommand(t *testing.T) {
	err := new(exec.PipedExec).
		Command("qqqqqqjkljlj", "hello").
		Run(os.Stdout, os.Stdout)
	require.Error(t, err)
}

func Test_EmptyCommandList(t *testing.T) {
	err := new(exec.PipedExec).
		Run(os.Stdout, os.Stdout)
	require.Error(t, err)
}

func Test_KillProcessUsingFirst(t *testing.T) {
	pe := new(exec.PipedExec)
	pe.Command("sleep", "10")
	cmd := pe.GetCmd(0)

	go func() {
		defer fmt.Println("Bye")
		<-time.After(300 * time.Millisecond)
		fmt.Println("Killing process...")
		_ = cmd.Process.Kill()
	}()

	fmt.Println("Running...")
	err := pe.Run(os.Stdout, os.Stderr)
	fmt.Println("err=", err)
	assert.NotNil(t, err)
}

func Test_RunToStrings(t *testing.T) {
	{
		stdouts, stderrs, err := new(exec.PipedExec).
			Command("sh", "-c", "echo 11").
			RunToStrings()
		assert.Nil(t, err)
		assert.Equal(t, "11", strings.TrimSpace(stdouts))
		assert.Equal(t, "", stderrs)
	}

	// 1 > &2
	{
		stdouts, stderrs, err := new(exec.PipedExec).
			Command("sh", "-c", "echo 11 1>&2").
			RunToStrings()
		assert.Nil(t, err)
		assert.Equal(t, "11", strings.TrimSpace(stderrs))
		assert.Equal(t, "", stdouts)
	}

	//stdout and stderr
	{
		stdouts, stderrs, err := new(exec.PipedExec).
			Command("sh", "-c", "echo err 1>&2; echo std").
			RunToStrings()
		assert.Nil(t, err)
		assert.Equal(t, "std", strings.TrimSpace(stdouts))
		assert.Equal(t, "err", strings.TrimSpace(stderrs))
	}

	//Wrong command
	{
		stdouts, stderrs, err := new(exec.PipedExec).
			Command("itmustbeawrongcommandPipedExecRunToStrings").
			RunToStrings()
		assert.NotNil(t, err)
		assert.Equal(t, "", stdouts)
		assert.Equal(t, "", stderrs)
	}

}
