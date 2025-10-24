#!/bin/bash
# LDAPLite Startup Script - Clean & Safe

set -e

echo "🧹 Cleaning up any existing processes..."
pkill -9 -f "ldaplite server" 2>/dev/null || true
pkill -9 -f "bin/ldaplite" 2>/dev/null || true

echo "⏳ Waiting for port 3389 to clear..."
sleep 3

echo "📁 Setting up database directory..."
mkdir -p /tmp/ldaplite_data

echo "🚀 Starting LDAPLite server..."
export LDAP_BASE_DN="${LDAP_BASE_DN:-dc=example,dc=com}"
export LDAP_ADMIN_PASSWORD="${LDAP_ADMIN_PASSWORD:-admin123}"
export LDAP_DATABASE_PATH="${LDAP_DATABASE_PATH:-/tmp/ldaplite_data/ldaplite.db}"
export LDAP_PORT="${LDAP_PORT:-3389}"

echo "📋 Configuration:"
echo "  Base DN: $LDAP_BASE_DN"
echo "  Port: $LDAP_PORT"
echo "  Database: $LDAP_DATABASE_PATH"
echo ""

# Run in foreground (use 'nohup ./bin/ldaplite server &' to background)
./bin/ldaplite server
