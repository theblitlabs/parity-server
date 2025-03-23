# Parity Protocol

Parity Protocol is a decentralized compute network where runners can execute compute tasks (e.g., running a Docker file) and earn incentives in the form of tokens. Task creators can add tasks to a pool, and the first runner to complete a task successfully receives a reward.

## Quick Start

### Prerequisites

- Go 1.22.7 or higher (using Go toolchain 1.23.4)
- PostgreSQL
- Make

### Installation

1. Clone the repository:

```bash
git clone https://github.com/theblitlabs/parity-server.git
cd parity-server
```

2. Install dependencies:

```bash
make deps
```

3. Start PostgreSQL (if using Docker):

```bash
# Remove existing container if it exists
docker rm -f parity-db || true

# Start new PostgreSQL container
docker run --name parity-db -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres
```

### Development

The project includes several helpful Makefile commands for development:

```bash
make build          # Build the application
make run           # Run the application
make clean         # Clean build files
make deps          # Download dependencies
make fmt           # Format code using gofumpt (preferred) or gofmt
make imports       # Fix imports formatting
make format        # Run all formatters (gofumpt + goimports)
make lint          # Run linters
make format-lint   # Format code and run linters in one step
make check-format  # Check code formatting without applying changes
make install-lint-tools # Install formatting and linting tools
make watch         # Run with hot reload (requires air)
make install       # Install parity command globally
make uninstall     # Remove parity command from system
make help          # Display all available commands
```

For hot reloading during development:

```bash
# Install air (required for hot reloading)
make install-air

# Run with hot reload
make watch
```

### Configuration

Create a `config.yaml` file in the `config` directory using the example provided:

```yaml
ethereum:
  rpc: "http://localhost:8545"
  chain_id: 1337
  token_address: "0x..."
  stake_wallet_address: "0x..."

server:
  host: "localhost"
  port: "8080"
  endpoint: "/api"

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  name: "parity"
  sslmode: "disable"
```

### CLI Commands

The CLI provides a unified interface through the `parity` command:

```bash
parity-server
```

Each command supports the `--help` flag for detailed usage information:

```bash
parity-server <command> --help
```

### API Documentation

#### Task Endpoints

| Method | Endpoint               | Description      |
| ------ | ---------------------- | ---------------- |
| POST   | /api/tasks             | Create task      |
| GET    | /api/tasks             | List all tasks   |
| GET    | /api/tasks/{id}        | Get task details |
| PUT    | /api/tasks/{id}        | Update task      |
| DELETE | /api/tasks/{id}        | Delete task      |
| GET    | /api/tasks/{id}/status | Get task status  |
| GET    | /api/tasks/{id}/logs   | Get task logs    |
| GET    | /api/tasks/{id}/reward | Get task reward  |

#### Runner Endpoints

| Method | Endpoint                         | Description           |
| ------ | -------------------------------- | --------------------- |
| POST   | /api/runners/register            | Register new runner   |
| GET    | /api/runners/tasks/available     | List available tasks  |
| POST   | /api/runners/tasks/{id}/claim    | Claim task            |
| POST   | /api/runners/tasks/{id}/start    | Start task execution  |
| POST   | /api/runners/tasks/{id}/complete | Complete task         |
| POST   | /api/runners/tasks/{id}/fail     | Mark task as failed   |
| GET    | /api/runners/stats               | Get runner statistics |
| POST   | /api/runners/heartbeat           | Send heartbeat        |

### Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Install git hooks for pre-commit checks (`make install-hooks`)
4. Commit your changes (`git commit -m 'Add some amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

### License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
