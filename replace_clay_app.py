import os

def main():
    target = "clay-app"
    replacement = "clay-tubes"
    root_dir = r"d:\zicofarry\GitHub\clay-tubes"

    exclude_dirs = {".git", ".idea", ".vscode", "vendor"}
    processed_files = 0
    modified_files = []

    for dirpath, dirnames, filenames in os.walk(root_dir):
        # Prune directory tree in place to avoid walking into excluded dirs
        dirnames[:] = [d for d in dirnames if d not in exclude_dirs]
        
        for filename in filenames:
            # Do not process our own script
            if filename == "replace_clay_app.py":
                continue
                
            filepath = os.path.join(dirpath, filename)
            
            # Skip potential binary files by checking extension
            ext = os.path.splitext(filename)[1].lower()
            if ext in {'.exe', '.png', '.jpg', '.jpeg', '.gif', '.ico', '.pdf', '.zip', '.tar', '.gz', '.dll', '.so', '.dylib', '.db', '.keystore', '.apk', '.aab'}:
                continue
                
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
                
                if target in content:
                    new_content = content.replace(target, replacement)
                    with open(filepath, 'w', encoding='utf-8') as f:
                        f.write(new_content)
                    modified_files.append(filepath)
                    print(f"Updated: {os.path.relpath(filepath, root_dir)}")
                processed_files += 1
            except Exception as e:
                print(f"Error processing {filepath}: {e}")

    print(f"\nSummary:")
    print(f"Total processed files: {processed_files}")
    print(f"Total modified files: {len(modified_files)}")

if __name__ == "__main__":
    main()
