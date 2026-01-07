# dis-imf-uploader

An IMF file upload service with review workflow, AWS S3/CloudFront integration, Cloudflare cache management, and notifications(slack).

## Features

### Core Functionality

- ✅ File Upload: Upload files to temporary storage pending review
- ✅ Review Process: Reviewer approves or rejects with optional notes
- ✅ S3 Integration: Automatic upload with backup of previous versions
- ✅ CloudFront Invalidation: Automatic cache invalidation after approval
- ✅ Cloudflare Purge: Manual cache purge via API
- ✅ Slack Notifications: Real-time notifications for key events
- ✅ Audit Logging: Complete operation history in MongoDB


## Getting started

### Dependencies

### Configuration

## Contributing

See [CONTRIBUTING](CONTRIBUTING.md) for details.

## License

Copyright © 2026, Office for National Statistics (<https://www.ons.gov.uk>)

Released under MIT license, see [LICENSE](LICENSE.md) for details.
