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
            return output[:2000] if output else "Command executed successfully (no output)."
        else:
            err_output = stderr.decode('utf-8', errors='replace')
            return f"Command Failed:\n{err_output}"
            
    except Exception as e:
        return f"Error executing command: {str(e)}"
