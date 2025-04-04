# Parity Protocol

A decentralized compute network enabling trustless task execution with token incentives. Task creators can submit compute tasks (e.g., Docker containers, scripts) to a pool, while runners compete to execute them for rewards. Built with Go and blockchain technology for transparent, secure, and efficient distributed computing.

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Development](#development)
- [Configuration](#configuration)
- [CLI Usage](#cli-usage)
- [API Documentation](#api-documentation)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Quick Start

### Prerequisites

- Go 1.22.7 or higher (using Go toolchain 1.23.4)
- PostgreSQL 14.0 or higher
- Make
- Docker (optional, for containerized database)
- Git

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

3. Set up the database:

Using Docker (recommended for development):

```bash
# Remove existing container if it exists
docker rm -f parity-db || true

# Start new PostgreSQL container
docker run --name parity-db \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=parity \
  -p 5432:5432 \
  -d postgres:14
```

Or using local PostgreSQL:

```bash
createdb parity
```

4. Create and configure your environment:

```bash
cp config/config.example.yaml config/config.yaml
# Edit config/config.yaml with your settings
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

Create a `config.yaml` file in the `config` directory using the example provided. Make sure to replace the placeholder values with your own configuration:

```yaml
server:
  port: # The port your server will listen on (e.g. 8080)
  host: # The host to bind to (e.g. localhost)
  endpoint: # API endpoint prefix (e.g. /api)

database:
  url: postgres://<username>:<password>@<host>:<port>/<database_name> # Your PostgreSQL connection URL

ethereum:
  rpc: # Your Ethereum RPC endpoint (e.g. http://localhost:8545)
  chain_id: # Your blockchain network ID (e.g. 1337 for local, 1 for mainnet)
  token_address: # The deployed token contract address
  stake_wallet_address: # The wallet address used for staking

aws:
  region: # AWS region for your services (e.g. us-east-1)
  bucket_name: # AWS S3 bucket name for storage

scheduler:
  interval: # Task scheduler interval (e.g. 10, 30, 60)
```

> **Important**: The above values are examples only. You must replace them with your own configuration values before running the application. Particularly important are:
>
> - The database URL with your PostgreSQL credentials
> - The Ethereum RPC endpoint for your network
> - The correct chain ID for your network
> - Your deployed token contract address
> - Your stake wallet address
> - AWS credentials and region if using S3 storage
> - Appropriate scheduler interval for your needs

### CLI Usage

The CLI provides a unified interface through the `parity-server` command:

```bash
# Start the server
parity-server

# View all available commands
parity-server --help

# Get help for a specific command
parity-server <command> --help
```

Common commands:

```bash
parity-server          # Start the server
parity-server version  # Show version information
parity-server config   # Show current configuration
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

Please ensure your PR:

- Includes tests for new features
- Updates documentation as needed
- Follows the existing code style
- Includes a clear description of changes

### License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
