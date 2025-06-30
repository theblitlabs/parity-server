# Parity Server

The core orchestration server for the PLGenesis decentralized AI and compute network. Parity Server handles task distribution, LLM request routing, federated learning coordination, runner management, and blockchain interactions. It provides a robust REST API for clients and manages the entire network coordination.

## 🚀 Features

### 🤖 LLM Infrastructure

- **Model Discovery**: Automatic detection and listing of available LLM models across runners
- **Async Processing**: Non-blocking prompt submission with real-time status tracking
- **Smart Routing**: Intelligent distribution of LLM requests to capable runners
- **Token Economics**: Comprehensive billing and reward mechanisms for LLM inference

### 🧠 Federated Learning Coordination

- **Session Management**: Create, coordinate, and monitor distributed federated learning sessions
- **Participant Auto-Selection**: Automatic assignment of online runners to FL sessions
- **Data Partitioning**: Server-side coordination of 5 data partitioning strategies
- **Model Aggregation**: FedAvg and other aggregation algorithms with customizable methods
- **Round Management**: Automatic progression through training rounds with status tracking
- **Privacy Controls**: Support for differential privacy and secure aggregation
- **Requirements Validation**: Strict validation of all training parameters with no default values

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
- [Federated Learning](#federated-learning)
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
git clone https://github.com/virajbhartiya/parity-server.git
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

## Federated Learning

The parity-server coordinates federated learning sessions with strict requirements validation and no default values.

### Key Capabilities

#### 🎯 Requirements-Based System

- **No Default Values**: All training parameters must be explicitly provided by clients
- **Strict Validation**: Comprehensive parameter validation with clear error messages
- **Configuration Required**: Model architecture must be specified via client configuration files

#### 🔄 Session Coordination

- **Automatic Participant Assignment**: Server assigns unique participant indices to runners
- **Round Management**: Automatic progression through training rounds
- **Status Tracking**: Real-time monitoring of session and participant status
- **Model Aggregation**: Server performs FedAvg aggregation with configurable methods

#### 📊 Data Partitioning Support

The server coordinates data partitioning across participants:

1. **Random (IID)**: Uniform random distribution
2. **Stratified**: Maintains class distribution across participants
3. **Sequential**: Consecutive data splits
4. **Non-IID**: Dirichlet distribution for realistic heterogeneity
5. **Label Skew**: Each participant gets subset of classes

#### 🛡️ Validation & Safety

- **Model Config Validation**: Ensures all required model parameters are provided
- **Training Parameter Validation**: Validates learning rates, batch sizes, epochs
- **Partition Strategy Validation**: Strategy-specific requirement checking
- **NaN Protection**: Built-in safeguards against numerical instability

### FL Session Lifecycle

1. **Session Creation**: Client provides complete configuration
2. **Participant Assignment**: Server auto-assigns online runners with unique indices
3. **Data Partitioning**: Server coordinates partitioning based on strategy
4. **Training Tasks**: Server creates tasks with participant-specific configurations
5. **Model Updates**: Runners submit weights and gradients
6. **Aggregation**: Server performs FedAvg aggregation
7. **Round Progression**: Automatic advancement to next round or completion

### Configuration Requirements

All FL sessions require explicit configuration:

```json
{
  "session_config": {
    "aggregation_method": "federated_averaging",
    "learning_rate": 0.001,
    "batch_size": 32,
    "local_epochs": 5
  },
  "model_config": {
    "input_size": 784,
    "hidden_size": 128,
    "output_size": 10
  },
  "partition_config": {
    "strategy": "non_iid",
    "alpha": 0.5,
    "min_samples": 100,
    "overlap_ratio": 0.0
  }
}
```

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

#### Federated Learning Endpoints

| Method | Endpoint                                       | Description          | Requirements                     |
| ------ | ---------------------------------------------- | -------------------- | -------------------------------- |
| POST   | /api/v1/federated-learning/sessions            | Create FL session    | Complete model + training config |
| GET    | /api/v1/federated-learning/sessions            | List FL sessions     | -                                |
| GET    | /api/v1/federated-learning/sessions/{id}       | Get session details  | -                                |
| POST   | /api/v1/federated-learning/sessions/{id}/start | Start FL session     | -                                |
| GET    | /api/v1/federated-learning/sessions/{id}/model | Get trained model    | Session completed                |
| POST   | /api/v1/federated-learning/model-updates       | Submit model updates | Valid weights + gradients        |

#### Create FL Session Request Example

```json
{
  "name": "MNIST Classification",
  "description": "Distributed digit classification",
  "model_type": "neural_network",
  "total_rounds": 10,
  "min_participants": 3,
  "creator_address": "0x123...",
  "training_data": {
    "dataset_cid": "QmYourDatasetCID",
    "data_format": "csv",
    "split_strategy": "non_iid",
    "metadata": {
      "alpha": 0.5,
      "min_samples": 100,
      "overlap_ratio": 0.0
    }
  },
  "config": {
    "aggregation_method": "federated_averaging",
    "learning_rate": 0.001,
    "batch_size": 32,
    "local_epochs": 5,
    "model_config": {
      "input_size": 784,
      "hidden_size": 128,
      "output_size": 10
    }
  }
}
```

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

## Security

### Federated Learning Security

- **Input Validation**: All FL parameters are strictly validated
- **Configuration Verification**: Model configs must be explicitly provided
- **Participant Authentication**: Secure runner verification for FL sessions
- **Data Isolation**: Each participant only accesses their data partition
- **Aggregation Security**: Server-side validation of model updates

### General Security

- **Authentication**: Secure API access with proper validation
- **Data Protection**: IPFS/Filecoin integration for secure data storage
- **Network Security**: Blockchain integration for transparent operations

## Troubleshooting

### Common Issues

1. **Federated Learning Issues**

   - **Invalid model configuration**: Ensure complete model config is provided
   - **Missing training parameters**: All training parameters must be explicitly set
   - **Partition validation errors**: Check strategy-specific requirements
   - **No participants available**: Ensure runners are online and registered

2. **Database Connection Issues**

   - Check PostgreSQL is running and accessible
   - Verify database credentials in `.env` file
   - Ensure database exists and migrations are applied

3. **Runner Registration Issues**

   - Verify runner webhook endpoints are accessible
   - Check heartbeat monitoring configuration
   - Ensure proper network connectivity

4. **Docker Issues**
   - Ensure Docker and Docker Compose are installed
   - Check port conflicts (default: 8080, 5432)
   - Verify environment variables are properly set

### Error Examples

**FL Configuration Error**:

```
model configuration is required - please provide model parameters
```

**Solution**: Ensure client provides complete model configuration via API

**Partition Validation Error**:

```
alpha parameter is required for non_iid partitioning strategy
```

**Solution**: Provide alpha parameter in training data metadata for non_iid strategy

## Contributing

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

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
