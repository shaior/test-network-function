package interactive_test

import (
	"errors"
	"github.com/golang/mock/gomock"
	expect "github.com/google/goexpect"
	"github.com/redhat-nfvpe/test-network-function/pkg/tnf/interactive"
	mock_interactive "github.com/redhat-nfvpe/test-network-function/pkg/tnf/interactive/mocks"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
	"time"
)

// Note: Test coverage for this file is as high as possible short of attempting to perform multi-threaded unit tests.
// Some lines cannot be covered as they are specifically geared towards production use v.s. unit test use (mock injection).

const (
	testTimeoutDuration = time.Second * 2
)

func init() {
	interactive.UnitTestMode = true
}

var (
	defaultGoExpectArgs            = []expect.Option{expect.Verbose(true)}
	defaultStdout, defaultStdin, _ = os.Pipe()
	startError                     = errors.New("start failed")
	stdinPipeError                 = errors.New("failed to access stdin")
	stdoutPipeError                = errors.New("failed to access stdout")
)

type goExpectSpawnerTestCase struct {
	goExpectSpawnerSpawnCommand string
	goExpectSpawnerSpawnArgs    []string
	goExpectSpawnerSpawnTimeout time.Duration
	goExpectSpawnerSpawnOpts    []expect.Option

	stdinPipeShouldBeCalled bool
	stdinPipeReturnValue    io.WriteCloser
	stdinPipeReturnErr      error

	stdoutPipeShouldBeCalled bool
	stdoutPipeReturnValue    io.Reader
	stdoutPipeReturnErr      error

	startShouldBeCalled bool
	startReturnErr      error

	goExpectSpawnerSpawnReturnContextIsNil bool
	goExpectSpawnerSpawnReturnErr          error
}

var goExpectSpawnerTestCases = map[string]goExpectSpawnerTestCase{
	// 1. Test to ensure that if StdinPipe() fails, that the error cascades back out of the Spawn invocation.
	"stdin_pipe_creation_failure": {
		// The command is unimportant
		goExpectSpawnerSpawnCommand: "ls",
		goExpectSpawnerSpawnArgs:    []string{"-al"},
		goExpectSpawnerSpawnTimeout: testTimeoutDuration,
		goExpectSpawnerSpawnOpts:    defaultGoExpectArgs,

		// Sets up a scenario in which the call to StdinPipe() returns an error.  This error should cascade out of Spawn().
		stdinPipeShouldBeCalled: true,
		stdinPipeReturnValue:    nil,
		stdinPipeReturnErr:      stdinPipeError,

		stdoutPipeShouldBeCalled: false,
		stdoutPipeReturnValue:    nil,
		stdoutPipeReturnErr:      nil,

		startShouldBeCalled: false,
		startReturnErr:      nil,

		goExpectSpawnerSpawnReturnContextIsNil: true,
		goExpectSpawnerSpawnReturnErr:          stdinPipeError,
	},
	// 2. Progressing past the creation of stdin, now cause stdout to fail.
	"stdout_pipe_creation_failure": {
		// The command is unimportant
		goExpectSpawnerSpawnCommand: "ls",
		goExpectSpawnerSpawnArgs:    []string{"-al"},
		goExpectSpawnerSpawnTimeout: testTimeoutDuration,
		goExpectSpawnerSpawnOpts:    defaultGoExpectArgs,

		stdinPipeShouldBeCalled: true,
		stdinPipeReturnValue:    defaultStdin,
		stdinPipeReturnErr:      nil,

		// cause StdoutPipe() call to fail and ensure the error cascades.
		stdoutPipeShouldBeCalled: true,
		stdoutPipeReturnValue:    nil,
		stdoutPipeReturnErr:      stdoutPipeError,

		startShouldBeCalled: false,
		startReturnErr:      nil,

		goExpectSpawnerSpawnReturnContextIsNil: true,
		goExpectSpawnerSpawnReturnErr:          stdoutPipeError,
	},
	// 3. Progressing past the creation of stdin/stdout, now cause Start to fail.
	"start_failure": {
		// The command is unimportant
		goExpectSpawnerSpawnCommand: "ls",
		goExpectSpawnerSpawnArgs:    []string{"-al"},
		goExpectSpawnerSpawnTimeout: testTimeoutDuration,
		goExpectSpawnerSpawnOpts:    defaultGoExpectArgs,

		stdinPipeShouldBeCalled: true,
		stdinPipeReturnValue:    defaultStdin,
		stdinPipeReturnErr:      nil,

		stdoutPipeShouldBeCalled: true,
		stdoutPipeReturnValue:    defaultStdout,
		stdoutPipeReturnErr:      nil,

		// cause Start() call to fail and make sure the error cascades out of Spawn().
		startShouldBeCalled: true,
		startReturnErr:      startError,

		goExpectSpawnerSpawnReturnContextIsNil: true,
		goExpectSpawnerSpawnReturnErr:          startError,
	},
	// 4. Successful spawn.
	"successful_spawn": {
		// The command is unimportant
		goExpectSpawnerSpawnCommand: "ls",
		goExpectSpawnerSpawnArgs:    []string{"-al"},
		goExpectSpawnerSpawnTimeout: testTimeoutDuration,
		goExpectSpawnerSpawnOpts:    defaultGoExpectArgs,

		stdinPipeShouldBeCalled: true,
		stdinPipeReturnValue:    defaultStdin,
		stdinPipeReturnErr:      nil,

		stdoutPipeShouldBeCalled: true,
		stdoutPipeReturnValue:    defaultStdout,
		stdoutPipeReturnErr:      nil,

		startShouldBeCalled: true,
		startReturnErr:      nil,

		goExpectSpawnerSpawnReturnContextIsNil: false,
		goExpectSpawnerSpawnReturnErr:          nil,
	},
}

func TestGoExpectSpawner_Spawn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, testCase := range goExpectSpawnerTestCases {
		mockSpawnFunc := mock_interactive.NewMockSpawnFunc(ctrl)
		// coax the types
		var sFunc interactive.SpawnFunc = mockSpawnFunc
		interactive.SetSpawnFunc(&sFunc)

		if testCase.stdinPipeShouldBeCalled {
			mockSpawnFunc.EXPECT().StdinPipe().Return(testCase.stdinPipeReturnValue, testCase.stdinPipeReturnErr)
		}

		if testCase.stdoutPipeShouldBeCalled {
			mockSpawnFunc.EXPECT().StdoutPipe().Return(testCase.stdoutPipeReturnValue, testCase.stdoutPipeReturnErr)
		}

		if testCase.startShouldBeCalled {
			mockSpawnFunc.EXPECT().Start().Return(testCase.startReturnErr)
		}

		// Wait() is executed within the expect.Expect.waitForSession(...) function, and is done so through a separate
		// goroutine.  We can't make any expectations of this thread, as doing so is prone to race conditions.  Take
		// the simple way out, and just allow Wait() to be invoked any number of times.
		mockSpawnFunc.EXPECT().Wait().AnyTimes()

		// Command is always called...
		mockSpawnFunc.EXPECT().Command(testCase.goExpectSpawnerSpawnCommand, testCase.goExpectSpawnerSpawnArgs).Return(&sFunc)

		goExpectSpawner := interactive.NewGoExpectSpawner()
		context, err := goExpectSpawner.Spawn(testCase.goExpectSpawnerSpawnCommand, testCase.goExpectSpawnerSpawnArgs, testCase.goExpectSpawnerSpawnTimeout, testCase.goExpectSpawnerSpawnOpts...)
		assert.Equal(t, testCase.goExpectSpawnerSpawnReturnErr, err)
		assert.Equal(t, testCase.goExpectSpawnerSpawnReturnContextIsNil, context == nil)
	}
}

// Also tests GetExpecter() and GetErrorChannel().
func TestNewContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockExpecter := mock_interactive.NewMockExpecter(ctrl)
	var errorChannel <-chan error
	var expecter expect.Expecter = mockExpecter
	context := interactive.NewContext(&expecter, errorChannel)
	assert.Equal(t, &expecter, context.GetExpecter())
	assert.Equal(t, errorChannel, context.GetErrorChannel())
}

func TestExecSpawnFunc(t *testing.T) {
	execSpawnFunc := interactive.ExecSpawnFunc{}
	cmd := execSpawnFunc.Command("pwd")
	assert.NotNil(t, cmd)

	stdin, err := (*cmd).StdinPipe()
	assert.Nil(t, err)
	assert.NotNil(t, stdin)

	stdout, err := (*cmd).StdoutPipe()
	assert.Nil(t, err)
	assert.NotNil(t, stdout)

	err = (*cmd).Start()
	assert.Nil(t, err)

	err = (*cmd).Wait()
	assert.Nil(t, err)
}