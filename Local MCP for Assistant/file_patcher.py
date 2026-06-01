"""File patching tools extracted for MCP."""
import asyncio
import difflib
import os
from concurrent.futures import ProcessPoolExecutor
import config

def _read_file_worker(filepath: str) -> str:
    """Worker function to read a file with line numbers."""
    safe_path = os.path.basename(filepath)
    full_path = os.path.join(config.SCRIPTS_DIR, safe_path)

    if not os.path.exists(full_path):
        return f"ERROR: File '{safe_path}' does not exist."

    try:
        with open(full_path, "r", encoding="utf-8") as f:
            lines = f.readlines()

        if not lines:
            return f"File '{safe_path}' is empty."

        numbered_lines = []
        for i, line in enumerate(lines, start=1):
            numbered_lines.append(f"{i:>4}| {line.rstrip()}")

        return f"--- {safe_path} ({len(lines)} lines) ---\n" + "\n".join(
            numbered_lines
        )
    except Exception as e:
        return f"ERROR: Could not read file: {str(e)}"


def _apply_patch_worker(
    filepath: str, old_text: str, new_text: str, action: str
) -> str:
    """Worker function to create, edit (with difflib), or delete files."""
    safe_path = os.path.basename(filepath)
    full_path = os.path.join(config.SCRIPTS_DIR, safe_path)

    action = action.lower().strip()

    # --- ACTION: CREATE ---
    if action == "create":
        if os.path.exists(full_path):
            return f"ERROR: File '{safe_path}' already exists. Use action='edit' or 'delete'."
        try:
            if not os.path.exists(config.SCRIPTS_DIR):
                os.makedirs(config.SCRIPTS_DIR)
            with open(full_path, "w", encoding="utf-8") as f:
                f.write(new_text)
            line_count = new_text.count("\n") + (1 if new_text else 0)
            return (
                f"SUCCESS: Created '{safe_path}' ({line_count} lines, "
                f"{len(new_text)} chars)."
            )
        except Exception as e:
            return f"ERROR: Could not create file: {str(e)}"

    # --- ACTION: DELETE ---
    if action == "delete":
        if not os.path.exists(full_path):
            return f"ERROR: File '{safe_path}' does not exist."
        try:
            os.remove(full_path)
            return f"SUCCESS: Deleted file '{safe_path}'."
        except Exception as e:
            return f"ERROR: Could not delete file: {str(e)}"

    # --- ACTION: EDIT (default) ---
    if action != "edit":
        return f"ERROR: Unknown action '{action}'. Use 'create', 'edit', or 'delete'."

    if not os.path.exists(full_path):
        return f"ERROR: File '{safe_path}' does not exist. Use action='create' to make it."

    if not old_text:
        return (
            "ERROR: 'old_text' is required for edit action. "
            "Provide the exact text segment to find and replace."
        )

    try:
        with open(full_path, "r", encoding="utf-8") as f:
            original_content = f.read()

        if not original_content:
            return "ERROR: File is empty. Use action='create' or action='edit' with a non-empty file."

        # 1. Try exact match first (most efficient)
        match_start = original_content.find(old_text)
        if match_start != -1:
            match_end = match_start + len(old_text)
        else:
            # 2. Try exact match with stripped lines (handles indentation/newline differences)
            # This is a bit more complex but still much faster than SequenceMatcher
            orig_lines = original_content.splitlines(keepends=True)
            old_lines = old_text.splitlines()
            
            best_start_idx = -1
            best_line_count = -1
            
            # Simple heuristic: try to find a block of lines that matches after stripping
            for i in range(len(orig_lines) - len(old_lines) + 1):
                match = True
                for j in range(len(old_lines)):
                    if orig_lines[i+j].strip() != old_lines[j].strip():
                        match = False
                        break
                if match:
                    best_start_idx = i
                    best_line_count = len(old_lines)
                    break
            
            if best_start_idx != -1:
                match_start = sum(len(l) for l in orig_lines[:best_start_idx])
                match_end = sum(len(l) for l in orig_lines[:best_start_idx + best_line_count])
            else:
                # 3. Fallback to difflib.SequenceMatcher but with limited scope
                matcher = difflib.SequenceMatcher(None, original_content, old_text)
                match = matcher.find_longest_match(0, len(original_content), 0, len(old_text))
                
                if match.size >= max(len(old_text) * 0.8, 10): # Require 80% match or at least 10 chars
                    match_start = match.a
                    match_end = match.a + match.size
                else:
                    return (
                        f"ERROR: Could not find a reliable match for the provided old_text.\n"
                        f"Please ensure you've copied the text exactly from the file.\n"
                        f"Hint: Provide more context or check for small typos."
                    )

        # Perform the edit
        new_content = (
            original_content[:match_start]
            + new_text
            + original_content[match_end:]
        )

        # Generate a unified diff for review
        original_lines_list = original_content.splitlines(keepends=True)
        new_lines_list = new_content.splitlines(keepends=True)

        diff_lines = list(
            difflib.unified_diff(
                original_lines_list,
                new_lines_list,
                fromfile=f"a/{safe_path}",
                tofile=f"b/{safe_path}",
                lineterm="",
            )
        )

        # Write the new content
        with open(full_path, "w", encoding="utf-8") as f:
            f.write(new_content)

        if diff_lines:
            diff_summary = "\n".join(diff_lines)
            return (
                f"SUCCESS: Edited '{safe_path}'.\n\n"
                f"--- PATCH APPLIED (unified diff) ---\n"
                f"{diff_summary}\n"
                f"--- END DIFF ---"
            )
        else:
            return f"SUCCESS: Edited '{safe_path}' (no visible diff — file unchanged)."

    except Exception as e:
        return f"ERROR: Patch failed: {str(e)}"


def _list_files_worker(directory: str = "") -> str:
    """Worker function to list files in the scripts directory."""
    if not os.path.exists(config.SCRIPTS_DIR):
        return f"No directory found at {config.SCRIPTS_DIR}."

    try:
        target_dir = config.SCRIPTS_DIR
        if directory:
            # Remove leading/trailing slashes and prevent path traversal
            safe_subdir = directory.strip("/\\")
            if ".." in safe_subdir:
                 return f"ERROR: Invalid directory path '{directory}'."
            target_dir = os.path.join(config.SCRIPTS_DIR, safe_subdir)

        if not os.path.exists(target_dir):
            return f"ERROR: Directory '{directory}' does not exist in {config.SCRIPTS_DIR}."

        all_files = []
        for root, dirs, files in os.walk(target_dir):
            rel_root = os.path.relpath(root, config.SCRIPTS_DIR)
            if rel_root == ".":
                rel_root = ""
            for filename in files:
                if rel_root:
                    all_files.append(f"  {rel_root}/{filename}")
                else:
                    all_files.append(f"  {filename}")
            for dirname in dirs:
                if rel_root:
                    all_files.append(f"  {rel_root}/{dirname}/")
                else:
                    all_files.append(f"  {dirname}/")

        if not all_files:
            return f"No files found in {config.SCRIPTS_DIR}/{directory}."

        return f"Files in {config.SCRIPTS_DIR}:\n" + "\n".join(sorted(all_files))
    except Exception as e:
        return f"ERROR: Could not list files: {str(e)}"

async def read_file(filepath: str) -> str:
    """
    Reads a file and returns its content with line numbers.
    Args:
        filepath: Path relative to the scripts directory.
    """
    return await asyncio.to_thread(_read_file_worker, filepath)

async def apply_patch(
    filepath: str,
    old_text: str = "",
    new_text: str = "",
    action: str = "edit",
) -> str:
    """
    Create, edit, or delete files using difflib for precise matching.
    Actions: 'create', 'edit', 'delete'.
    """
    return await asyncio.to_thread(_apply_patch_worker, filepath, old_text, new_text, action)

async def list_files(directory: str = "") -> str:
    """
    Lists all files in the scripts directory.
    """
    return await asyncio.to_thread(_list_files_worker, directory)

