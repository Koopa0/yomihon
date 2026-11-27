package confdir

// ProcessAlive says whether a process with the id is running. It is here
// because the architecture check lets this package alone name the calls that
// ask after another process.
//
// On Unix, signal 0 delivers nothing and reports only whether it could be
// delivered; a process that is not ours answers permission denied, which is
// still a process. On Windows a process object outlives its process for as long
// as any handle to it is open, anyone's, so being able to open one says
// nothing: it is running while a wait on it that waits for nothing runs out.
// There, one that cannot be opened is gone.
func ProcessAlive(pid int) bool {
	return pid >= 1 && running(pid)
}
