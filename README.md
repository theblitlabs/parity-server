# Parity Server

The core orchestration server for the PLGenesis decentralized AI and compute network. Parity Server handles task distribution, LLM request routing, runner management, and blockchain interactions. It provides a robust REST API for clients and manages the entire network coordination.

## 🚀 Features

### 🤖 LLM Infrastructure

- **Model Discovery**: Automatic detection and listing of available LLM models across runners
- **Async Processing**: Non-blocking prompt submission with real-time status tracking
- **Smart Routing**: Intelligent distribution of LLM requests to capable runners
- **Token Economics**: Comprehensive billing and reward mechanisms for LLM inference

### ⚡ Compute Task Management

- **Task Distribution**: Efficient routing of compute tasks to available runners
- **Status Tracking**: Real-time monitoring of task progress and completion
- **Load Balancing**: Intelligent workload distribution based on runner capabilities
- **Error Recovery**: Robust handling of failures and automatic retry mechanisms

### 🔒 Network Coordination

- **Runner Registration**: Secure onboarding and capability reporting
- **Heartbeat Monitoring**: Automatic detection of offline runners
- **Webhook Management**: Real-time task notifications and status updates
- **Blockchain Integration**: Transparent verification and reward distribution

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
# Filecoin Network Configuration
FILECOIN_CHAIN_ID=314159                                                         # Filecoin Calibration testnet
FILECOIN_RPC=https://api.calibration.node.glif.io/rpc/v1
FILECOIN_STAKE_WALLET_ADDRESS=0x7465e7a637f66cb7b294b856a25bc84abff1d247        # Stake wallet address
FILECOIN_TOKEN_ADDRESS=0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0              # Token contract address

# Database Configuration
DATABASE_USERNAME=postgres
DATABASE_PASSWORD=postgres
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_DATABASE_NAME=parity

# Filecoin Storage Configuration
FILECOIN_IPFS_ENDPOINT=http://localhost:5001
FILECOIN_GATEWAY_URL=https://gateway.pinata.cloud
FILECOIN_CREATE_STORAGE_DEALS=false

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
> - Network: Filecoin Calibration Testnet (Chain ID: 314159)
> - Token Contract: [`0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0`](https://filfox.info/en/address/0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0)
> - Stake Wallet: [`0x7465e7a637f66cb7b294b856a25bc84abff1d247`](https://filfox.info/en/address/0x7465e7a637f66cb7b294b856a25bc84abff1d247)
>
> For production deployment, you should replace these values with your own:
>
> - Database credentials
> - Filecoin RPC endpoint
> - Network chain ID
> - Token contract address
> - Stake wallet address
> - Filecoin/IPFS endpoints and gateway
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

#### LLM Endpoints

| Method | Endpoint                         | Description                        |
| ------ | -------------------------------- | ---------------------------------- |
| GET    | `/api/llm/models`                | List all available LLM models      |
| POST   | `/api/llm/prompts`               | Submit a prompt for LLM processing |
| GET    | `/api/llm/prompts/{id}`          | Get prompt status and response     |
| GET    | `/api/llm/prompts`               | List recent prompts                |
| POST   | `/api/llm/prompts/{id}/complete` | Complete prompt (internal use)     |
| GET    | `/api/llm/billing/metrics`       | Get billing metrics for client     |

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

#### Federated Learning Endpoints

| Method | Endpoint                                       | Description          |
| ------ | ---------------------------------------------- | -------------------- |
| POST   | /api/v1/federated-learning/sessions            | Create FL session    |
| GET    | /api/v1/federated-learning/sessions            | List FL sessions     |
| GET    | /api/v1/federated-learning/sessions/{id}       | Get session details  |
| POST   | /api/v1/federated-learning/sessions/{id}/start | Start FL session     |
| GET    | /api/v1/federated-learning/sessions/{id}/model | Get trained model    |
| POST   | /api/v1/federated-learning/model-updates       | Submit model updates |

#### Storage Endpoints

| Method | Endpoint                    | Description                  |
| ------ | --------------------------- | ---------------------------- |
| POST   | /api/storage/upload         | Upload file to IPFS/Filecoin |
| GET    | /api/storage/download/{cid} | Download file by CID         |
| GET    | /api/storage/info/{cid}     | Get file information         |
| POST   | /api/storage/pin/{cid}      | Pin file to IPFS             |

#### Health & Status Endpoints

| Method | Endpoint    | Description   |
| ------ | ----------- | ------------- |
| GET    | /api/health | Health check  |
| GET    | /api/status | System status |

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
