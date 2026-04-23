# 🔐 KeyVault Manager

KeyVault Manager is a macOS desktop application built with [Fyne](https://fyne.io/) for managing Azure Key Vault secrets across multiple subscriptions.

The app provides an easy-to-use graphical interface that wraps the [Azure CLI (`az`)](https://learn.microsoft.com/en-us/cli/azure/), allowing you to quickly browse vaults, read secrets, view history, and manage secrets without remembering complex terminal commands.

## Features

- **Subscription Management**: Automatically loads available Azure subscriptions and lets you seamlessly switch between them.
- **Vault Exploring**: Lists Key Vaults available in the selected subscription.
- **Secret Management**:
  - View all secrets within a selected Key Vault.
  - Create and push new secrets easily.
  - View the revision history of individual secrets.
- **macOS Native Feel**: Bundled as a `.app` making it easy to run securely like any native application.

## Prerequisites

Because KeyVault Manager uses Azure CLI commands under the hood, you need to have the Azure CLI installed and authenticated.

1. **Install Azure CLI**: 
   ```bash
   brew update && brew install azure-cli
   ```
2. **Authenticate**:
   ```bash
   az login
   ```

## Installation & Running Locally

1. **Clone the repository**:
   ```bash
   git clone https://github.com/eugenegoncharuk/keyvault-manager.git
   cd keyvault-manager
   ```

2. **Install dependencies** (assuming Go 1.21+ is installed):
   ```bash
   go mod download
   ```

3. **Run the App**:
   ```bash
   go run .
   ```

## Packaging for macOS

To build the macOS `.app` bundle locally:
1. Install the `fyne` CLI tool:
   ```bash
   go install fyne.io/fyne/v2/cmd/fyne@latest
   ```
2. Package the app:
   ```bash
   fyne package -os darwin -icon Icon.png
   ```
This will generate `KeyVault Manager.app` in your project folder, which you can drag to your Applications folder or distribute.

## GitHub Actions Release

This repository is configured with a GitHub Actions workflow that automatically builds and releases the macOS `.app` bundle whenever a new tag (e.g., `v1.0.0`) is pushed to the repository. The release will contain a `.tar.gz` archive with the built application, which can be downloaded directly from the GitHub Releases page.
