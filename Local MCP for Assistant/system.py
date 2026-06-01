"""System tools for terminal commands extracted for MCP."""
import asyncio
import config

# Configuration from config.py
FORBIDDEN_COMMANDS = config.FORBIDDEN_COMMANDS
COMMAND_TIMEOUT = config.COMMAND_TIMEOUT

async def run_terminal_command(command: str) -> str:
    """
    Executes a shell command asynchronously and returns the output.
    
    This function spawns a subprocess to run the command, capturing both 
    standard output (stdout) and standard error (stderr). It enforces a 
    timeout to prevent hanging processes.

    Args:
        command (str): The valid shell command to execute. 
                       Example: 'ls -la', 'grep "error" logs.txt'
    Returns:
        str: The command's standard output if successful, or the error message.
    """
    # Security Check
    if any(bad in command for bad in FORBIDDEN_COMMANDS):
        return f"ERROR: Security Block. Forbidden command."

    try:
        process = await asyncio.create_subprocess_shell(
            command,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE
        )

        try:
            stdout, stderr = await asyncio.wait_for(process.communicate(), timeout=COMMAND_TIMEOUT)
        except asyncio.TimeoutError:
            process.kill()
            await process.communicate()
            return f"Error: Command timed out after {COMMAND_TIMEOUT} seconds."

        if process.returncode == 0:
            output = stdout.decode('utf-8', errors='replace')
            if not output:
                return "Command executed successfully (no output)."
            
            # Increase limit to 8000 characters for more reliability
            if len(output) > 8000:
                return output[:8000] + "\n\n[Output truncated due to length...]"
            return output
        else:
            err_output = stderr.decode('utf-8', errors='replace')
            out_output = stdout.decode('utf-8', errors='replace')
            combined = f"Command Failed (exit code {process.returncode}):\n{err_output}"
            if out_output:
                combined += f"\n\nStandard Output:\n{out_output}"
            return combined[:8000]
            
    except Exception as e:
        return f"Error executing command: {str(e)}"
