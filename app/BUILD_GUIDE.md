# Build Guide

This guide explains how to build Enchanted for different environments and platforms.

## Build Configurations

### Production Builds

Production builds are published to the main S3 bucket and are available to all users.

**Configuration:**

- App ID: `com.eternis.enchanted`
- Product Name: `Enchanted`
- S3 Bucket: `enchanted-app`
- Channel: `latest`

**Build Commands:**

```bash
# macOS
pnpm run build:mac

# Windows
pnpm run build:win

# Linux
pnpm run build:linux
```

### Development/Pre-release Builds

Development builds are published to a separate S3 bucket and are only available to dev users.

**Configuration:**

- App ID: `com.eternis.enchanted.dev`
- Product Name: `Enchanted Dev`
- S3 Bucket: `enchanted-app-dev`
- Channel: `dev`
- Config: `build:dev` in `package.json`

**Build Commands:**

```bash
# macOS
pnpm run build:dev:mac

# Windows
pnpm run build:dev:win

# Linux
pnpm run build:dev:linux
```

## Using the Build Helper Script

For convenience, you can use the build helper script:

```bash
# Build production for macOS
pnpm run build:helper mac prod

# Build dev for macOS
pnpm run build:helper mac dev

# Build production for Windows
pnpm run build:helper win prod

# Build dev for Windows
pnpm run build:helper win dev
```

## Auto-Updater Behavior

The auto-updater automatically detects which channel to use based on the app name:

- **Enchanted Dev** → Uses `dev` channel (checks `enchanted-app-dev` bucket)
- **Enchanted** → Uses `latest` channel (checks `enchanted-app` bucket)

## S3 Bucket Setup

### Required Buckets

You need the following S3 buckets configured:

1. **enchanted-app** - Production releases
2. **enchanted-app-dev** - Development releases

Both buckets should be in the `us-east-1` region.

### S3 Bucket Configuration

#### 1. Bucket Policy

Add this bucket policy to allow public read access for update files:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PublicReadGetObject",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::your-bucket-name/*"
    }
  ]
}
```

#### 2. CORS Configuration

Add CORS configuration to allow cross-origin requests:

```json
[
  {
    "AllowedHeaders": ["*"],
    "AllowedMethods": ["GET", "HEAD"],
    "AllowedOrigins": ["*"],
    "ExposeHeaders": []
  }
]
```

#### 3. Block Public Access

Disable "Block all public access" settings for the buckets since electron-updater needs public read access.

### AWS Credentials

Make sure your AWS credentials are configured for uploading builds:

```bash
# Configure AWS CLI
aws configure

# Or set environment variables
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_DEFAULT_REGION=us-east-1
```

## Next Steps After S3 Setup

### 1. Test Your First Build

```bash
# Test production build
pnpm run build:mac

# Test dev build
pnpm run build:dev:mac
```

### 2. Verify Upload to S3

Check that your builds are uploaded to the correct buckets:

- Production builds → `enchanted-app` bucket
- Dev builds → `enchanted-app-dev` bucket

### 3. Test Auto-Updater

1. Install a dev version: `pnpm run build:dev:mac`
2. Increment version in `package.json` (e.g., `0.0.1` → `0.0.2`)
3. Build a newer dev version: `pnpm run build:dev:mac`
4. The installed dev app should detect and offer the update

### 4. CI/CD Integration

#### GitHub Actions Workflows

We have three workflows set up:

1. **`release.yml`** - Production releases (triggered by GitHub releases)
2. **`dev-release.yml`** - Dev releases for macOS (triggered by pushes to `develop`, `dev`, or `feature/*` branches)
3. **`dev-release-multi.yml`** - Multi-platform dev releases (manual trigger)

#### Required Secrets

Add these secrets to your GitHub repository:

```bash
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
CSC_LINK=your_macos_certificate
CSC_KEY_PASSWORD=your_certificate_password
APPLE_API_KEY=your_apple_api_key
NOTARY_API_KEY_ID=your_notary_key_id
NOTARY_API_ISSUER=your_notary_issuer
NOTARY_TEAM_ID=your_team_id
```

#### Workflow Triggers

- **Production**: Create a GitHub release or use workflow dispatch
- **Dev (macOS)**: Push to `develop`, `dev`, or `feature/*` branches
- **Dev (multi-platform)**: Manual workflow dispatch with platform selection

#### Environment-Specific Builds

The workflows automatically use the correct build commands:

```bash
# Production builds (release.yml)
pnpm run build:mac

# Dev builds (dev-release.yml)
pnpm run build:dev:mac

# Multi-platform dev builds (dev-release-multi.yml)
pnpm run build:dev:win
pnpm run build:dev:linux
```

## Version Management

- Production builds use the version from `package.json`
- Dev builds append `-dev` to the version in the artifact names
- Both builds can coexist on the same machine since they have different app IDs

## Testing Updates

1. Build a dev version: `pnpm run build:dev:mac`
2. Install the dev version
3. Build a newer dev version with an incremented version number
4. The dev app should automatically detect and offer the update

## Notes

- Dev builds have a different app ID, so they won't interfere with production installations
- Users can have both production and dev versions installed simultaneously
- The auto-updater respects the channel configuration automatically

## Troubleshooting

### Common Issues

#### Build Fails to Upload to S3

- Check AWS credentials are configured correctly
- Verify bucket names match exactly: `enchanted-app` and `enchanted-app-dev`
- Ensure bucket region is `us-east-1`

#### Auto-Updater Not Working

- Check that bucket policy allows public read access
- Verify CORS configuration is set correctly
- Ensure the app name matches exactly: "Enchanted" vs "Enchanted Dev"

#### Dev Builds Not Detecting Updates

- Make sure you're incrementing the version in `package.json`
- Verify the dev build is using the correct app ID: `com.eternis.enchanted.dev`
- Check that builds are uploaded to the `enchanted-app-dev` bucket

#### Both Apps Installing to Same Location

- This shouldn't happen since they have different app IDs
- If it does, check that the `appId` in `build:dev` config is correct

Logs will be available in:

- macOS: `~/Library/Logs/Enchanted/main.log`
- Windows: `%USERPROFILE%\AppData\Roaming\Enchanted\logs\main.log`
