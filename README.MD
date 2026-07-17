# Cloud Haunter

`Haunter is a pokémon ghost which will poison and kill your unidentified and orphan cloud resources`
 
The main goal of this project is to save costs in the cloud by monitoring and terminating resources.
Cloud Haunter (ch) supports the three major cloud providers.

`The project is under construction, use at your own risk!`

## Resource support matrix for quick overview
---

|          | AWS                                                  | AZURE           | GCP                            |
|----------|------------------------------------------------------|-----------------|--------------------------------|
| Stack    | Cloudformation stack, Native stack assembled by tags | Resource group  | Native stack assembled by tags |
| Instance | EC2 instance                                         | Virtual machine | Compute Engine instances       |
| Disk     | EC2 disk                                             | -               | Compute Engine disks           |
| Access   | IAM user                                             | -               | IAM service accounts           |
| Database | RDS database                                         | PG Flexible     | SQL instances                  |
| Alert    | CloudWatch alarm                                     | -               | -                              |
| Storage  | S3                                                   | Storage account | Cloud storage                  |

[For futher information please read the manual.](docs/MANUAL.md)

[For developers](docs/DEVELOPMENT.md)

*All documentation is located in the `docs/` folder.*
