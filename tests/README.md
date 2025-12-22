# ZWFM Aeron API Tests

This directory contains test fixtures and data for the ZWFM Aeron API project.

## Structure

```
tests/
├── fixtures/           # Test data and configuration
│   ├── mock_data.sql   # Mock database data
│   └── test_config.json # Test configuration
└── docker-compose.test.yml # Test database setup (optional for local testing)
```

## Test Execution

All tests are executed through GitHub Actions. See `.github/workflows/comprehensive-test.yml` for the complete test suite.

## Local Testing (Optional)

If you want to test locally, you can:

### 1. Start Test Database

```bash
cd tests
docker-compose -f docker-compose.test.yml up -d
```

This starts a PostgreSQL container on port 5433 with mock data.

### 2. Run the application manually

```bash
go build -o zwfm-aeronapi .
./zwfm-aeronapi -config=tests/fixtures/test_config.json
```

## Test Data

The `fixtures/mock_data.sql` file contains:
- 80 popular artists (international and Dutch)
  - 10 artists with dummy JPEG images (Queen, ABBA, Madonna, etc.)
  - 70 artists without images
- 130+ tracks with realistic data
  - 10 tracks with dummy JPEG images (Bohemian Rhapsody, Dancing Queen, etc.)
  - 120+ tracks without images

## CI/CD Integration

GitHub Actions automatically:
1. Sets up a test database
2. Loads mock data
3. Runs all integration tests
4. Reports results

## Writing New Tests

1. Add test data to `fixtures/`
2. Create test scripts in `integration/`
3. Update GitHub Actions workflow if needed

## Test Configuration

The test configuration uses:
- **Database**: PostgreSQL 16
- **Port**: 5432 (CI) or 5433 (local)
- **API Keys**: test-api-key-12345, another-test-key-67890