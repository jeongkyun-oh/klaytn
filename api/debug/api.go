package debug

import (
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"os"
	"strings"
	"os/user"
	"path/filepath"
	"sync"
	"time"
	"github.com/ground-x/go-gxplatform/log"
	"github.com/ground-x/go-gxplatform/metrics"
	"github.com/ground-x/go-gxplatform/metrics/exp"
	"errors"
	"io"
)

// Handler is the global debugging handler.
var Handler = new(HandlerT)
var logger = log.NewModuleLogger(log.APIDebug)

// HandlerT implements the debugging API.
// Do not create values of this type, use the one
// in the Handler variable instead.
type HandlerT struct {
	mu        sync.Mutex
	cpuW      io.WriteCloser
	cpuFile   string
	memFile   string
	traceW    io.WriteCloser
	traceFile string

	// For the pprof http server
	handlerInited bool
	pprofServer   *http.Server
}

// Verbosity sets the log verbosity ceiling. The verbosity of individual packages
// and source files can be raised using Vmodule.
func (*HandlerT) Verbosity(level int) error {
	return log.ChangeGlobalLogLevel(glogger, log.Lvl(level))
}

// VerbosityByName sets the verbosity of log module with given name.
// Please note that VerbosityByName only works with zapLogger.
func (*HandlerT) VerbosityByName(mn string, level int) error {
	return log.ChangeLogLevelWithName(mn, log.Lvl(level))
}

// VerbosityByID sets the verbosity of log module with given ModuleID.
// Please note that VerbosityByID only works with zapLogger.
func (*HandlerT) VerbosityByID(mi int, level int) error {
	return log.ChangeLogLevelWithID(log.ModuleID(mi), log.Lvl(level))
}

// Vmodule sets the log verbosity pattern. See package log for details on the
// pattern syntax.
func (*HandlerT) Vmodule(pattern string) error {
	return glogger.Vmodule(pattern)
}

// BacktraceAt sets the log backtrace location. See package log for details on
// the pattern syntax.
func (*HandlerT) BacktraceAt(location string) error {
	return glogger.BacktraceAt(location)
}

// MemStats returns detailed runtime memory statistics.
func (*HandlerT) MemStats() *runtime.MemStats {
	s := new(runtime.MemStats)
	runtime.ReadMemStats(s)
	return s
}

// GcStats returns GC statistics.
func (*HandlerT) GcStats() *debug.GCStats {
	s := new(debug.GCStats)
	debug.ReadGCStats(s)
	return s
}

func (h *HandlerT) StartPProf(address string, port int) error {
	// Set the default server address and port if they are not set
	if address == "" {
		address = pprofAddrFlag.Value
	}
	if port == 0 {
		port = pprofPortFlag.Value
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pprofServer != nil {
		return errors.New("pprof server is already running")
	}

	serverAddr := fmt.Sprintf("%s:%d", address, port)
	httpServer := &http.Server{Addr: serverAddr}

	if !h.handlerInited {
		// Hook go-metrics into expvar on any /debug/metrics request, load all vars
		// from the registry into expvar, and execute regular expvar handler.
		exp.Exp(metrics.DefaultRegistry)
		http.Handle("/memsize/", http.StripPrefix("/memsize", &Memsize))
		h.handlerInited = true
	}

	logger.Info("Starting pprof server", "addr", fmt.Sprintf("http://%s/debug/pprof", serverAddr))
	go func(handle *HandlerT) {
		if err := httpServer.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				logger.Info("pprof server is closed")
			} else {
				logger.Error("Failure in running pprof server", "err", err)
			}
		}
		h.mu.Lock()
		h.pprofServer = nil
		h.mu.Unlock()
	}(h)

	h.pprofServer = httpServer

	return nil
}

func (h *HandlerT) StopPProf() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pprofServer == nil {
		return errors.New("pprof server is not running")
	}

	logger.Info("Shutting down pprof server")
	h.pprofServer.Close()

	return nil
}

func (h *HandlerT) IsPProfRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pprofServer != nil
}

// CpuProfile turns on CPU profiling for nsec seconds and writes
// profile data to file.
func (h *HandlerT) CpuProfile(file string, nsec uint) error {
	if err := h.StartCPUProfile(file); err != nil {
		return err
	}
	time.Sleep(time.Duration(nsec) * time.Second)
	h.StopCPUProfile()
	return nil
}

// StartCPUProfile turns on CPU profiling, writing to the given file.
func (h *HandlerT) StartCPUProfile(file string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cpuW != nil {
		return errors.New("CPU profiling already in progress")
	}
	f, err := os.Create(expandHome(file))
	if err != nil {
		return err
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return err
	}
	h.cpuW = f
	h.cpuFile = file
	logger.Info("CPU profiling started", "dump", h.cpuFile)
	return nil
}

// StopCPUProfile stops an ongoing CPU profile.
func (h *HandlerT) StopCPUProfile() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	pprof.StopCPUProfile()
	if h.cpuW == nil {
		return errors.New("CPU profiling not in progress")
	}
	logger.Info("Done writing CPU profile", "dump", h.cpuFile)
	h.cpuW.Close()
	h.cpuW = nil
	h.cpuFile = ""
	return nil
}

// GoTrace turns on tracing for nsec seconds and writes
// trace data to file.
func (h *HandlerT) GoTrace(file string, nsec uint) error {
	if err := h.StartGoTrace(file); err != nil {
		return err
	}
	time.Sleep(time.Duration(nsec) * time.Second)
	h.StopGoTrace()
	return nil
}

// BlockProfile turns on goroutine profiling for nsec seconds and writes profile data to
// file. It uses a profile rate of 1 for most accurate information. If a different rate is
// desired, set the rate and write the profile manually.
func (*HandlerT) BlockProfile(file string, nsec uint) error {
	runtime.SetBlockProfileRate(1)
	time.Sleep(time.Duration(nsec) * time.Second)
	defer runtime.SetBlockProfileRate(0)
	return writeProfile("block", file)
}

// SetBlockProfileRate sets the rate of goroutine block profile data collection.
// rate 0 disables block profiling.
func (*HandlerT) SetBlockProfileRate(rate int) {
	runtime.SetBlockProfileRate(rate)
}

// WriteBlockProfile writes a goroutine blocking profile to the given file.
func (*HandlerT) WriteBlockProfile(file string) error {
	return writeProfile("block", file)
}

// MutexProfile turns on mutex profiling for nsec seconds and writes profile data to file.
// It uses a profile rate of 1 for most accurate information. If a different rate is
// desired, set the rate and write the profile manually.
func (*HandlerT) MutexProfile(file string, nsec uint) error {
	runtime.SetMutexProfileFraction(1)
	time.Sleep(time.Duration(nsec) * time.Second)
	defer runtime.SetMutexProfileFraction(0)
	return writeProfile("mutex", file)
}

// SetMutexProfileFraction sets the rate of mutex profiling.
func (*HandlerT) SetMutexProfileFraction(rate int) {
	runtime.SetMutexProfileFraction(rate)
}

// WriteMutexProfile writes a goroutine blocking profile to the given file.
func (*HandlerT) WriteMutexProfile(file string) error {
	return writeProfile("mutex", file)
}

// WriteMemProfile writes an allocation profile to the given file.
// Note that the profiling rate cannot be set through the API,
// it must be set on the command line.
func (*HandlerT) WriteMemProfile(file string) error {
	return writeProfile("heap", file)
}

// Stacks returns a printed representation of the stacks of all goroutines.
func (*HandlerT) Stacks() string {
	buf := make([]byte, 1024*1024)
	buf = buf[:runtime.Stack(buf, true)]
	return string(buf)
}

// FreeOSMemory returns unused memory to the OS.
func (*HandlerT) FreeOSMemory() {
	debug.FreeOSMemory()
}

// SetGCPercent sets the garbage collection target percentage. It returns the previous
// setting. A negative value disables GC.
func (*HandlerT) SetGCPercent(v int) int {
	return debug.SetGCPercent(v)
}

func writeProfile(name, file string) error {
	p := pprof.Lookup(name)
	logger.Info("Writing profile records", "count", p.Count(), "type", name, "dump", file)
	f, err := os.Create(expandHome(file))
	if err != nil {
		return err
	}
	defer f.Close()
	return p.WriteTo(f, 0)
}

// expands home directory in file paths.
// ~someuser/tmp will not be expanded.
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home := os.Getenv("HOME")
		if home == "" {
			if usr, err := user.Current(); err == nil {
				home = usr.HomeDir
			}
		}
		if home != "" {
			p = home + p[1:]
		}
	}
	return filepath.Clean(p)
}

