# Parity Protocol

A decentralized compute network enabling trustless task execution with token incentives. Task creators can submit compute tasks (e.g., Docker containers, scripts) to a pool, while runners compete to execute them for rewards. Built with Go and blockchain technology for transparent, secure, and efficient distributed computing.

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Development](#development)
- [Docker Setup](#docker-setup)
- [Configuration](#configuration)
- [CLI Usage](#cli-usage)
- [API Documentation](#api-documentation)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Quick Start

### Prerequisites

- Go 1.24 or higher (using Go toolchain 1.24)
- PostgreSQL 15.0 or higher
- Make
- Docker
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

3. Set up your environment:

```bash
cp .env.example .env
# Edit .env with your settings
```

### Development

The project includes several helpful Makefile commands for development:

```bash
# Build and Run
make build          # Build the application
make run           # Run the application
make clean         # Clean build files
make deps          # Download dependencies

# Code Quality
make fmt           # Format code using gofumpt (preferred) or gofmt
make imports       # Fix imports formatting
make format        # Run all formatters (gofumpt + goimports)
make lint          # Run linters
make format-lint   # Format code and run linters in one step
make check-format  # Check code formatting without applying changes

# Development Tools
make install-lint-tools # Install formatting and linting tools
make watch         # Run with hot reload (requires air)
make install       # Install parity command globally
make uninstall     # Remove parity command from system

# Docker Commands
make docker-build  # Build Docker image
make docker-up     # Start Docker containers
make docker-down   # Stop Docker containers
make docker-logs   # View Docker container logs
make docker-clean  # Remove Docker containers, images, and volumes
make docker-ps     # List running Docker containers
make docker-exec   # Execute command in Docker container

make help          # Display all available commands
```

### Docker Setup

The project includes a complete Docker setup for both development and production environments.

#### Using Docker Compose

1. Build and start the services:

```bash
make docker-build
make docker-up
```

2. View logs:

```bash
make docker-logs
```

3. Stop services:

```bash
make docker-down
```

The Docker setup includes:

- PostgreSQL 15-alpine database
- Parity server application
- Automatic database health checks
- Environment variable configuration

#### Environment Configuration

Create a `.env` file with the following configuration:

```env
# Ethereum Configuration
ETHEREUM_CHAIN_ID=11155111                                                         # Sepolia testnet
ETHEREUM_RPC=https://eth-sepolia.g.alchemy.com/v2/API_KEY
ETHEREUM_STAKE_WALLET_ADDRESS=0x261259e9467E042DBBF372906e17b94fC06942f2        # Stake wallet address
ETHEREUM_TOKEN_ADDRESS=0x844303bcC1a347bE6B409Ae159b4040d84876024              # Token contract address

# Database Configuration
DATABASE_USERNAME=postgres
DATABASE_PASSWORD=postgres
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_DATABASE_NAME=parity

# AWS Configuration
AWS_REGION=ap-south-1
AWS_BUCKET_NAME=dev-parity-docker-images

# Server Configuration
SERVER_PORT=8080
SERVER_ENDPOINT=/api
SERVER_HOST=0.0.0.0

# Runner Configuration
RUNNER_WEBHOOK_PORT=8080
RUNNER_API_PREFIX=/api
RUNNER_SERVER_URL=http://localhost:8080

# Scheduler Configuration
SCHEDULER_INTERVAL=10
```

> **Important**: The above configuration shows the current Sepolia testnet setup. Key contract details:
>
> - Network: Sepolia Testnet (Chain ID: 11155111)
> - Token Contract: [`0x844303bcC1a347bE6B409Ae159b4040d84876024`](https://sepolia.etherscan.io/address/0x844303bcC1a347bE6B409Ae159b4040d84876024)
> - Stake Wallet: [`0x261259e9467E042DBBF372906e17b94fC06942f2`](https://sepolia.etherscan.io/address/0x261259e9467E042DBBF372906e17b94fC06942f2)
>
> For production deployment, you should replace these values with your own:
>
> - Database credentials
> - Ethereum RPC endpoint
> - Network chain ID
> - Token contract address
> - Stake wallet address
> - AWS credentials and region
> - Scheduler interval

### CLI Usage

The CLI provides a unified interface through the `parity-server` command:

```bash
# Start the server
parity-server server

# Authenticate
parity-server auth

# Stake tokens
parity-server stake --amount 10

# Check balance
parity-server balance


# View all available commands
parity-server --help
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
