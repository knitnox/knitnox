"""System tools for terminal commands extracted for MCP."""
import asyncio
import config
import os
import signal

# Configuration from config.py
FORBIDDEN_COMMANDS = config.FORBIDDEN_COMMANDS
COMMAND_TIMEOUT = config.COMMAND_TIMEOUT

# Global state to track the active terminal process
_active_process = None
_active_command = None
_process_lock = asyncio.Lock()

async def _kill_process_tree(process):
    """
    Kills a process and all its children.
    On Windows, uses taskkill. On Unix, uses process groups.
    """
    if process is None or process.returncode is not None:
        return

    pid = process.pid
    try:
        if os.name == 'nt':
            # Windows: taskkill /F /T /PID <pid>
            # /F = Force, /T = Tree
            kill_proc = await asyncio.create_subprocess_shell(
                f"taskkill /F /T /PID {pid}",
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE
            )
            await kill_proc.wait()
        else:
            # Unix: Send SIGTERM then SIGKILL to the process group
            process.terminate()
            try:
                await asyncio.wait_for(process.wait(), timeout=2.0)
            except asyncio.TimeoutError:
                process.kill()
    except Exception as e:
        print(f"Error killing process tree for PID {pid}: {e}")

async def kill_process_on_port(port: int) -> str:
    """
    Finds and terminates any process listening on the specified port.
    
    Args:
        port (int): The port number to clear.
    Returns:
        str: Success or error message.
    """
    try:
        if os.name == 'nt':
            # Find PID using netstat
            cmd = f"netstat -ano | findstr LISTENING | findstr :{port}"
            proc = await asyncio.create_subprocess_shell(
                cmd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE
            )
            stdout, _ = await proc.communicate()
            output = stdout.decode().strip()
            
            if not output:
                return f"No process found listening on port {port}."
            
            # Extract PIDs (last column)
            pids = set()
            for line in output.splitlines():
                parts = line.split()
                if parts:
                    pids.add(parts[-1])
            
            results = []
            for pid in pids:
                kill_cmd = f"taskkill /F /T /PID {pid}"
                k_proc = await asyncio.create_subprocess_shell(kill_cmd, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
                await k_proc.wait()
                results.append(f"Killed PID {pid} on port {port}")
            
            return "\n".join(results)
        else:
            # Unix: lsof -ti:<port> | xargs kill -9
            cmd = f"lsof -ti:{port}"
            proc = await asyncio.create_subprocess_shell(cmd, stdout=asyncio.subprocess.PIPE)
            stdout, _ = await proc.communicate()
            pids = stdout.decode().strip().split()
            
            if not pids:
                return f"No process found listening on port {port}."
            
            for pid in pids:
                os.kill(int(pid), signal.SIGKILL)
            
            return f"Killed PIDs {', '.join(pids)} on port {port}."
            
    except Exception as e:
        return f"Error killing process on port {port}: {str(e)}"

async def stop_terminal_command() -> str:
    """
    Stops the currently running terminal process, if any.
    
    Returns:
        str: A message indicating whether a process was stopped or if none was running.
    """
    global _active_process, _active_command
    
    async with _process_lock:
        if _active_process is None:
            return "No terminal process is currently running."
            
        try:
            # Check if process is still alive
            if _active_process.returncode is None:
                cmd = _active_command
                await _kill_process_tree(_active_process)
                msg = f"Successfully stopped the running process tree: {cmd}"
            else:
                msg = f"Process '{_active_command}' had already finished with exit code {_active_process.returncode}."
                
            _active_process = None
            _active_command = None
            return msg
        except Exception as e:
            _active_process = None
            _active_command = None
            return f"Error while stopping process: {str(e)}"

async def run_terminal_command(command: str) -> str:
    """
    Executes a shell command asynchronously and returns the output.
    Only ONE terminal process can run at a time. Starting a new one will 
    automatically stop any previously running process.
    
    This function spawns a subprocess to run the command, capturing both 
    standard output (stdout) and standard error (stderr). It enforces a 
    timeout to prevent hanging processes.

    Args:
        command (str): The valid shell command to execute. 
                       Example: 'ls -la', 'grep "error" logs.txt'
    Returns:
        str: The command's standard output if successful, or the error message.
    """
    global _active_process, _active_command

    # Security Check
    if any(bad in command for bad in FORBIDDEN_COMMANDS):
        return f"ERROR: Security Block. Forbidden command."

    # Immediate interruption of previous process if it's still running
    if _active_process is not None and _active_process.returncode is None:
        try:
            # Use taskkill logic for internal interruption too
            p_to_kill = _active_process
            asyncio.create_task(_kill_process_tree(p_to_kill))
        except Exception:
            pass

    async with _process_lock:
        try:
            _active_command = command
            _active_process = await asyncio.create_subprocess_shell(
                command,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE
            )
            
            # Keep a local reference to this specific process
            current_p = _active_process

            try:
                stdout, stderr = await asyncio.wait_for(current_p.communicate(), timeout=COMMAND_TIMEOUT)
            except asyncio.TimeoutError:
                await _kill_process_tree(current_p)
                # Clear global state only if this is still the active process
                if _active_process is current_p:
                    _active_process = None
                    _active_command = None
                return f"Error: Command timed out after {COMMAND_TIMEOUT} seconds. The process tree has been terminated to prevent orphaning."
            except Exception as e:
                # If we were interrupted by another command killing us
                if current_p.returncode is not None:
                     pass # Handled below
                else:
                    raise e
            finally:
                # Clear global state only if this is still the active process
                if _active_process is current_p:
                    _active_process = None
                    _active_command = None

            ret_code = current_p.returncode
            if ret_code == 0:
                output = stdout.decode('utf-8', errors='replace')
                if not output:
                    return "Command executed successfully (no output)."
                
                if len(output) > 8000:
                    return output[:8000] + "\n\n[Output truncated due to length...]"
                return output
            elif ret_code == -9 or ret_code == 15 or (os.name == 'nt' and ret_code == 1): 
                # Note: taskkill usually results in exit code 1 or similar for the killed process on Windows
                return f"Command '{command}' was interrupted or stopped."
            else:
                err_output = stderr.decode('utf-8', errors='replace')
                out_output = stdout.decode('utf-8', errors='replace')
                combined = f"Command Failed (exit code {ret_code}):\n{err_output}"
                if out_output:
                    combined += f"\n\nStandard Output:\n{out_output}"
                return combined[:8000]
                
        except Exception as e:
            return f"Error executing command: {str(e)}"
