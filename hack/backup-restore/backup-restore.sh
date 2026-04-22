#!/bin/bash
# Unified Backup/Restore Tool for AI Services
# Usage: ./backup-restore.sh <command> [options]

set -e

VERSION="1.0.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_error() { echo -e "${RED} $1${NC}"; }
print_success() { echo -e "${GREEN} $1${NC}"; }
print_warning() { echo -e "${YELLOW}  $1${NC}"; }
print_info() { echo -e "${BLUE}ℹ $1${NC}"; }

# Show usage
show_usage() {
    cat << EOF
Unified Backup/Restore Tool for AI Services v${VERSION}

USAGE:
    ./backup-restore.sh <command> [options]

COMMANDS:
    export opensearch <app-name> <output-file>
        Export OpenSearch vector database
        Example: ./backup-restore.sh export opensearch rag-dev opensearch.tar.gz

    export digitize <output-file>
        Export digitize application data (/var/cache)
        Example: ./backup-restore.sh export digitize digitize.tar.gz

    import opensearch <backup-file>
        Import OpenSearch vector database
        Example: ./backup-restore.sh import opensearch opensearch.tar.gz

    import digitize <backup-file>
        Import digitize application data
        Example: ./backup-restore.sh import digitize digitize.tar.gz

    help
        Show this help message

    version
        Show version information

EXAMPLES:
    # Full backup (run both commands)
    ./backup-restore.sh export opensearch rag-dev opensearch.tar.gz
    ./backup-restore.sh export digitize digitize.tar.gz

    # Full restore (run both commands)
    ./backup-restore.sh import opensearch opensearch.tar.gz
    ./backup-restore.sh import digitize digitize.tar.gz

    # Partial backup (OpenSearch only)
    ./backup-restore.sh export opensearch rag-prod opensearch_prod.tar.gz

    # Partial restore (digitize only)
    ./backup-restore.sh import digitize digitize_backup.tar.gz

EOF
}

# Export OpenSearch
export_opensearch() {
    local APP_NAME="${1:-rag-dev}"
    local OUTPUT_FILE="${2:-opensearch_backup_$(date +%Y%m%d_%H%M%S).tar.gz}"
    local CONTAINER_NAME=$(podman ps | grep opensearch | awk '{print $1}')

    if [ -z "$CONTAINER_NAME" ]; then
        print_error "OpenSearch container not found"
        exit 1
    fi

    echo "============================================================"
    echo "OpenSearch Export"
    echo "============================================================"
    echo "Container: $CONTAINER_NAME"
    echo "App name: $APP_NAME"
    echo "Output: $OUTPUT_FILE"
    echo ""

    # Create Python backup script
    print_info "Creating backup script..."
    podman exec $CONTAINER_NAME bash -c 'cat > /tmp/backup.py << '\''EOFPYTHON'\''
#!/usr/bin/env python3
import json, os, sys, tarfile, tempfile
from datetime import datetime
from pathlib import Path
from opensearchpy import OpenSearch

class BackupExporter:
    def __init__(self, app_name, output_file):
        self.app_name = app_name
        self.output_file = output_file
        self.client = OpenSearch(
            hosts=[{"host": "localhost", "port": 9200}],
            http_compress=True, use_ssl=True,
            http_auth=("admin", os.getenv("OPENSEARCH_PASSWORD", "AiServices@12345")),
            verify_certs=False, ssl_show_warn=False, timeout=30
        )
    
    def export_index(self, index_name, temp_dir):
        print(f"  Exporting index: {index_name}")
        mapping = self.client.indices.get_mapping(index=index_name)
        settings = self.client.indices.get_settings(index=index_name)
        with open(temp_dir / f"{index_name}_mapping.json", "w") as f:
            json.dump(mapping, f)
        with open(temp_dir / f"{index_name}_settings.json", "w") as f:
            json.dump(settings, f)
        documents = []
        response = self.client.search(index=index_name, body={"query": {"match_all": {}},"size": 1000}, params={"scroll": "5m"})
        scroll_id = response["_scroll_id"]
        hits = response["hits"]["hits"]
        documents.extend(hits)
        while len(hits) > 0:
            response = self.client.scroll(scroll_id=scroll_id, params={"scroll": "5m"})
            scroll_id = response["_scroll_id"]
            hits = response["hits"]["hits"]
            documents.extend(hits)
        self.client.clear_scroll(scroll_id=scroll_id)
        with open(temp_dir / f"{index_name}_data.json", "w") as f:
            json.dump(documents, f)
        print(f"    ✓ {len(documents)} documents")
    
    def run(self):
        print("Connecting to OpenSearch...")
        info = self.client.info()
        print(f"✓ Connected to OpenSearch {info['\''version'\'']['\''number'\'']}")
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)
            os_dir = temp_path / "opensearch"
            os_dir.mkdir(exist_ok=True)
            indices = [idx for idx in self.client.indices.get_alias(index="*").keys() if idx.startswith("rag")]
            print(f"Found {len(indices)} indices")
            for idx in indices:
                self.export_index(idx, os_dir)
            with open(temp_path / "backup_info.json", "w") as f:
                json.dump({"app_name": self.app_name, "backup_date": datetime.now().isoformat(), "type": "opensearch"}, f)
            with tarfile.open(self.output_file, "w:gz") as tar:
                tar.add(temp_path, arcname="backup")
            size_mb = os.path.getsize(self.output_file) / (1024 * 1024)
            print(f"✓ Backup created: {self.output_file} ({size_mb:.2f} MB)")

if __name__ == "__main__":
    exporter = BackupExporter(sys.argv[1], sys.argv[2])
    exporter.run()
EOFPYTHON
'

    # Install dependencies
    print_info "Installing dependencies..."
    if ! podman exec $CONTAINER_NAME bash -c "command -v pip &> /dev/null || command -v pip3 &> /dev/null"; then
        podman exec --user root $CONTAINER_NAME bash -c "
            if command -v yum &> /dev/null; then
                yum install -y python3-pip 2>&1 | tail -3
            elif command -v dnf &> /dev/null; then
                dnf install -y python3-pip 2>&1 | tail -3
            elif command -v apt-get &> /dev/null; then
                apt-get update -qq && apt-get install -y python3-pip 2>&1 | tail -3
            elif command -v apk &> /dev/null; then
                apk add --no-cache py3-pip 2>&1 | tail -3
            else
                python3 -m ensurepip --default-pip 2>&1 | tail -3 || true
            fi
        " 2>/dev/null || true
    fi

    podman exec $CONTAINER_NAME bash -c "
        if command -v pip &> /dev/null; then
            pip install --user opensearch-py 2>&1 | grep -E '(Successfully installed|Requirement already satisfied)' || true
        elif command -v pip3 &> /dev/null; then
            pip3 install --user opensearch-py 2>&1 | grep -E '(Successfully installed|Requirement already satisfied)' || true
        elif python3 -m pip --version &> /dev/null; then
            python3 -m pip install --user opensearch-py 2>&1 | grep -E '(Successfully installed|Requirement already satisfied)' || true
        fi
    " 2>/dev/null

    # Run backup
    print_info "Running OpenSearch backup..."
    podman exec -e OPENSEARCH_PASSWORD=AiServices@12345 \
        $CONTAINER_NAME python3 /tmp/backup.py "$APP_NAME" /tmp/backup.tar.gz

    # Copy to host
    print_info "Copying backup to host..."
    podman cp $CONTAINER_NAME:/tmp/backup.tar.gz "./$OUTPUT_FILE"

    # Cleanup
    podman exec $CONTAINER_NAME rm -f /tmp/backup.py /tmp/backup.tar.gz

    echo ""
    print_success "OpenSearch export completed!"
    echo "Backup file: $OUTPUT_FILE"
    ls -lh "$OUTPUT_FILE"
}

# Export Digitize
export_digitize() {
    local OUTPUT_FILE="${1:-digitize_backup_$(date +%Y%m%d_%H%M%S).tar.gz}"
    local CACHE_DIR="/var/cache"

    echo "============================================================"
    echo "Digitize Data Export"
    echo "============================================================"
    echo "Cache directory: $CACHE_DIR"
    echo "Output: $OUTPUT_FILE"
    echo ""

    if [ ! -d "$CACHE_DIR" ]; then
        print_error "$CACHE_DIR directory not found on host"
        exit 1
    fi

    print_info "Creating backup of /var/cache..."
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"

    mkdir -p backup/cache
    cp -r "$CACHE_DIR"/* backup/cache/ 2>/dev/null || true

    TOTAL_FILES=$(find backup/cache -type f 2>/dev/null | wc -l)
    TOTAL_SIZE=$(du -sh backup/cache 2>/dev/null | awk '{print $1}')

    echo "  ✓ Backed up $TOTAL_FILES files ($TOTAL_SIZE) from host"

    tar -czf "$OLDPWD/$OUTPUT_FILE" backup/
    cd "$OLDPWD"
    rm -rf "$TEMP_DIR"

    echo ""
    print_success "Digitize data export completed!"
    echo "Backup file: $OUTPUT_FILE"
    ls -lh "$OUTPUT_FILE"
}


# Import OpenSearch
import_opensearch() {
    local BACKUP_FILE="$1"

    if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
        print_error "Backup file not found: $BACKUP_FILE"
        exit 1
    fi

    local CONTAINER_NAME=$(podman ps | grep opensearch | awk '{print $1}')

    if [ -z "$CONTAINER_NAME" ]; then
        print_error "OpenSearch container not found"
        exit 1
    fi

    echo "============================================================"
    echo "OpenSearch Import"
    echo "============================================================"
    echo "Container: $CONTAINER_NAME"
    echo "Backup file: $BACKUP_FILE"
    echo ""

    # Copy backup to container
    print_info "Copying backup to container..."
    podman cp "$BACKUP_FILE" $CONTAINER_NAME:/tmp/backup.tar.gz

    # Create restore script
    print_info "Creating restore script..."
    podman exec $CONTAINER_NAME bash -c 'cat > /tmp/restore.py << '\''EOFPYTHON'\''
#!/usr/bin/env python3
import json, os, sys, tarfile, tempfile
from pathlib import Path
from opensearchpy import OpenSearch, helpers

class BackupRestorer:
    def __init__(self, backup_file):
        self.backup_file = backup_file
        self.client = OpenSearch(
            hosts=[{"host": "localhost", "port": 9200}],
            http_compress=True, use_ssl=True,
            http_auth=("admin", os.getenv("OPENSEARCH_PASSWORD", "AiServices@12345")),
            verify_certs=False, ssl_show_warn=False, timeout=30
        )
    
    def restore_index(self, index_name, temp_dir):
        print(f"  Restoring index: {index_name}")
        os_dir = temp_dir / "backup" / "opensearch"
        with open(os_dir / f"{index_name}_mapping.json") as f:
            mapping = json.load(f)
        with open(os_dir / f"{index_name}_settings.json") as f:
            settings = json.load(f)
        if self.client.indices.exists(index=index_name):
            print(f"    Deleting existing index...")
            self.client.indices.delete(index=index_name)
        idx_settings = settings[index_name]["settings"]["index"]
        for key in ["creation_date", "uuid", "version", "provided_name"]:
            idx_settings.pop(key, None)
        self.client.indices.create(
            index=index_name,
            body={"settings": {"index": idx_settings}, "mappings": mapping[index_name]["mappings"]}
        )
        with open(os_dir / f"{index_name}_data.json") as f:
            documents = json.load(f)
        if documents:
            actions = [{"_index": index_name, "_id": doc["_id"], "_source": doc["_source"]} for doc in documents]
            success, errors = helpers.bulk(self.client, actions, stats_only=False, raise_on_error=False, refresh=True)
            print(f"    ✓ {success} documents restored")
    
    def run(self):
        print("Connecting to OpenSearch...")
        info = self.client.info()
        print(f"✓ Connected to OpenSearch {info['\''version'\'']['\''number'\'']}")
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)
            print("Extracting backup...")
            with tarfile.open(self.backup_file, "r:gz") as tar:
                tar.extractall(temp_path)
            info_file = temp_path / "backup" / "backup_info.json"
            if info_file.exists():
                with open(info_file) as f:
                    info = json.load(f)
                    print(f"  Backup date: {info.get('\''backup_date'\'')}")
                    print(f"  App name: {info.get('\''app_name'\'')}")
            os_dir = temp_path / "backup" / "opensearch"
            if os_dir.exists():
                indices = [f.stem.replace("_data", "") for f in os_dir.glob("*_data.json")]
                print(f"Found {len(indices)} indices to restore")
                for idx in indices:
                    self.restore_index(idx, temp_path)
            print("✓ Restore completed successfully")

if __name__ == "__main__":
    restorer = BackupRestorer(sys.argv[1])
    restorer.run()
EOFPYTHON
'

    # Install dependencies
    print_info "Installing dependencies..."
    if ! podman exec $CONTAINER_NAME bash -c "command -v pip &> /dev/null || command -v pip3 &> /dev/null"; then
        podman exec --user root $CONTAINER_NAME bash -c "
            if command -v yum &> /dev/null; then
                yum install -y python3-pip 2>&1 | tail -3
            elif command -v dnf &> /dev/null; then
                dnf install -y python3-pip 2>&1 | tail -3
            elif command -v apt-get &> /dev/null; then
                apt-get update -qq && apt-get install -y python3-pip 2>&1 | tail -3
            elif command -v apk &> /dev/null; then
                apk add --no-cache py3-pip 2>&1 | tail -3
            else
                python3 -m ensurepip --default-pip 2>&1 | tail -3 || true
            fi
        " 2>/dev/null || true
    fi

    podman exec $CONTAINER_NAME bash -c "
        if command -v pip &> /dev/null; then
            pip install --user opensearch-py 2>&1 | grep -E '(Successfully installed|Requirement already satisfied)' || true
        elif command -v pip3 &> /dev/null; then
            pip3 install --user opensearch-py 2>&1 | grep -E '(Successfully installed|Requirement already satisfied)' || true
        elif python3 -m pip --version &> /dev/null; then
            python3 -m pip install --user opensearch-py 2>&1 | grep -E '(Successfully installed|Requirement already satisfied)' || true
        fi
    " 2>/dev/null

    # Run restore
    print_info "Running OpenSearch restore..."
    podman exec -e OPENSEARCH_PASSWORD=AiServices@12345 \
        $CONTAINER_NAME python3 /tmp/restore.py /tmp/backup.tar.gz

    # Cleanup
    print_info "Cleaning up..."
    podman exec $CONTAINER_NAME rm -f /tmp/restore.py /tmp/backup.tar.gz

    echo ""
    print_success "OpenSearch import completed!"
}

# Import Digitize
import_digitize() {
    local BACKUP_FILE="$1"

    if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
        print_error "Backup file not found: $BACKUP_FILE"
        exit 1
    fi

    echo "============================================================"
    echo "Digitize Data Import"
    echo "============================================================"
    echo "Backup file: $BACKUP_FILE"
    echo ""

    local CACHE_DIR="/var/cache"
    local TEMP_DIR=$(mktemp -d)

    # Extract backup
    print_info "Restoring /var/cache directory..."
    tar -xzf "$BACKUP_FILE" -C "$TEMP_DIR"

    if [ -d "$TEMP_DIR/backup/cache" ]; then
        mkdir -p "$CACHE_DIR"
        cp -r "$TEMP_DIR/backup/cache"/* "$CACHE_DIR/" 2>/dev/null || true
        
        TOTAL_FILES=$(find "$CACHE_DIR" -type f 2>/dev/null | wc -l)
        TOTAL_SIZE=$(du -sh "$CACHE_DIR" 2>/dev/null | awk '{print $1}')
        echo "  ✓ Restored $TOTAL_FILES files ($TOTAL_SIZE) to $CACHE_DIR"
    else
        print_warning "No cache directory found in backup"
    fi

    rm -rf "$TEMP_DIR"

    # Copy to container
    print_info "Copying to digitize container..."
    local DIGITIZE_CONTAINER=$(podman ps --filter "name=digitize-backend" --format "{{.Names}}" | head -n 1)
    
    if [ -z "$DIGITIZE_CONTAINER" ]; then
        DIGITIZE_CONTAINER=$(podman ps --filter "name=digitize" --format "{{.Names}}" | head -n 1)
    fi

    if [ -n "$DIGITIZE_CONTAINER" ]; then
        echo "  ✓ Found container: $DIGITIZE_CONTAINER"
        
        podman exec $DIGITIZE_CONTAINER mkdir -p /var/cache 2>/dev/null || true
        
        tar -czf /tmp/cache_for_container.tar.gz -C /var cache 2>/dev/null
        podman cp /tmp/cache_for_container.tar.gz $DIGITIZE_CONTAINER:/tmp/ 2>/dev/null
        podman exec $DIGITIZE_CONTAINER tar -xzf /tmp/cache_for_container.tar.gz -C /var 2>/dev/null
        podman exec $DIGITIZE_CONTAINER rm -f /tmp/cache_for_container.tar.gz 2>/dev/null || true
        rm -f /tmp/cache_for_container.tar.gz
        
        CONTAINER_DOCS=$(podman exec $DIGITIZE_CONTAINER sh -c "ls -1 /var/cache/docs/*.json 2>/dev/null | wc -l" 2>/dev/null || echo "0")
        echo "  ✓ Container has $CONTAINER_DOCS document files"
    else
        print_warning "Digitize container not found"
    fi

    echo ""
    print_success "Digitize data import completed!"
    echo "🔄 Refresh your browser to see restored documents"
}


# Main command dispatcher
main() {
    if [ $# -eq 0 ]; then
        show_usage
        exit 1
    fi

    case "$1" in
        export)
            case "$2" in
                opensearch)
                    export_opensearch "${3:-rag-dev}" "${4:-opensearch_backup_$(date +%Y%m%d_%H%M%S).tar.gz}"
                    ;;
                digitize)
                    export_digitize "${3:-digitize_backup_$(date +%Y%m%d_%H%M%S).tar.gz}"
                    ;;
                *)
                    print_error "Unknown export target: $2"
                    echo "Valid targets: opensearch, digitize"
                    exit 1
                    ;;
            esac
            ;;
        import)
            case "$2" in
                opensearch)
                    if [ -z "$3" ]; then
                        print_error "Backup file required"
                        echo "Usage: ./backup-restore.sh import opensearch <backup-file>"
                        exit 1
                    fi
                    import_opensearch "$3"
                    ;;
                digitize)
                    if [ -z "$3" ]; then
                        print_error "Backup file required"
                        echo "Usage: ./backup-restore.sh import digitize <backup-file>"
                        exit 1
                    fi
                    import_digitize "$3"
                    ;;
                *)
                    print_error "Unknown import target: $2"
                    echo "Valid targets: opensearch, digitize"
                    exit 1
                    ;;
            esac
            ;;
        help|--help|-h)
            show_usage
            ;;
        version|--version|-v)
            echo "Backup/Restore Tool v${VERSION}"
            ;;
        *)
            print_error "Unknown command: $1"
            echo ""
            show_usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"

# Made with Bob
