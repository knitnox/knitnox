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

        # Find the best match for old_text using difflib.SequenceMatcher
        matcher = difflib.SequenceMatcher(None, original_content, old_text)
        match = matcher.find_longest_match(
            0, len(original_content), 0, len(old_text)
        )

        # Heuristic: if the match ratio is very low, try a more flexible approach
        if match.size < max(len(old_text) * 0.5, 5):
            # Try matching line-by-line for better partial matches
            original_lines = original_content.splitlines(keepends=True)
            old_lines = old_text.strip().splitlines()

            best_start = -1
            best_end = -1
            best_ratio = 0.0

            # Slide over the file looking for the best contiguous block match
            for i in range(len(original_lines)):
                for j in range(i + 1, len(original_lines) + 1):
                    candidate_block = "".join(original_lines[i:j])
                    line_matcher = difflib.SequenceMatcher(
                        None, candidate_block.strip(), old_text.strip()
                    )
                    ratio = line_matcher.ratio()
                    # Prefer longer matches with decent ratio
                    score = ratio * (j - i)
                    if score > best_ratio and ratio > 0.4:
                        best_ratio = score
                        best_start = i
                        best_end = j

            if best_start >= 0 and best_ratio > 0.4:
                match_start = sum(
                    len(line) for line in original_lines[:best_start]
                )
                match_end = sum(len(line) for line in original_lines[:best_end])
            else:
                return (
                    f"ERROR: Could not find a good match for the provided old_text. "
                    f"The file has {len(original_content)} chars. "
                    "Use read_file_agent first to inspect the file content, "
                    "then provide the exact text you want to replace."
                )
        else:
            match_start = match.a
            match_end = match.a + match.size

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

