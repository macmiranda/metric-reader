# EFS Emergency Mode Plugin

This plugin switches an Amazon EFS (Elastic File System) filesystem from bursting throughput mode to elastic throughput mode when metric thresholds are exceeded. This is designed to be used as an emergency action to prevent filesystem performance degradation or stalls when bursting credits are depleted.

## Use Case

When monitoring EFS burst credits (e.g., using YACE to collect `BurstCreditBalance` metrics from CloudWatch), you can trigger this plugin when credits fall below a critical threshold. Instead of performing I/O operations that would further deplete credits (like the `file_action` plugin), this plugin switches the filesystem to elastic throughput mode, which provides consistent baseline performance without relying on burst credits.

**Important:** This plugin only switches **to** elastic throughput mode. Returning the filesystem to bursting mode is out of scope and must be done manually or through other automation.

## Prerequisites

### 1. EFS Filesystem

- You need an existing EFS filesystem
- The filesystem must be in a state that allows throughput mode changes (typically `available` state)
- Note the filesystem ID (e.g., `fs-0123456789abcdef0`)

### 2. AWS Credentials

The plugin supports multiple authentication methods via the AWS SDK's default credential chain:

1. **IRSA (IAM Roles for Service Accounts)** - Recommended for EKS
   - Automatically detected when running on EKS with proper pod annotations
   - No explicit credentials needed in the pod

2. **EC2 Instance Profile**
   - Automatically detected when running on EC2 instances
   
3. **Environment Variables**
   - `AWS_ACCESS_KEY_ID`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_SESSION_TOKEN` (optional, for temporary credentials)

4. **Shared Credentials File**
   - `~/.aws/credentials`

### 3. IAM Permissions

The IAM role or user must have the following permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "elasticfilesystem:UpdateFileSystem",
        "elasticfilesystem:DescribeFileSystems"
      ],
      "Resource": "arn:aws:elasticfilesystem:REGION:ACCOUNT_ID:file-system/FILE_SYSTEM_ID"
    }
  ]
}
```

Replace:
- `REGION` with your AWS region (e.g., `us-east-1`)
- `ACCOUNT_ID` with your AWS account ID
- `FILE_SYSTEM_ID` with your EFS filesystem ID (e.g., `fs-0123456789abcdef0`)

For a more permissive policy (not recommended for production):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "elasticfilesystem:UpdateFileSystem",
        "elasticfilesystem:DescribeFileSystems"
      ],
      "Resource": "*"
    }
  ]
}
```

## Configuration

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `EFS_FILE_SYSTEM_ID` | The EFS filesystem ID to manage | `fs-0123456789abcdef0` |

### Optional Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `AWS_REGION` | AWS region where the EFS filesystem is located | Auto-detected from AWS config | `us-east-1` |

## Setting Up IRSA on EKS

To use IAM Roles for Service Accounts (IRSA) on EKS:

### 1. Create an IAM OIDC Provider for Your Cluster

```bash
eksctl utils associate-iam-oidc-provider \
  --cluster YOUR_CLUSTER_NAME \
  --approve
```

### 2. Create an IAM Role for the Service Account

Create a trust policy file (`trust-policy.json`):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/oidc.eks.REGION.amazonaws.com/id/OIDC_ID"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "oidc.eks.REGION.amazonaws.com/id/OIDC_ID:sub": "system:serviceaccount:NAMESPACE:SERVICE_ACCOUNT_NAME",
          "oidc.eks.REGION.amazonaws.com/id/OIDC_ID:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
```

Create the IAM role:

```bash
aws iam create-role \
  --role-name metric-reader-efs-emergency \
  --assume-role-policy-document file://trust-policy.json
```

### 3. Attach the EFS Policy to the Role

First, create the policy file (`efs-emergency-policy.json`) with the permissions shown above, then:

```bash
aws iam create-policy \
  --policy-name EFSEmergencyPolicy \
  --policy-document file://efs-emergency-policy.json

aws iam attach-role-policy \
  --role-name metric-reader-efs-emergency \
  --policy-arn arn:aws:iam::ACCOUNT_ID:policy/EFSEmergencyPolicy
```

### 4. Annotate the Kubernetes Service Account

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: metric-reader
  namespace: default
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT_ID:role/metric-reader-efs-emergency
```

### 5. Reference the Service Account in Your Pod

```yaml
spec:
  serviceAccountName: metric-reader
  containers:
  - name: metric-reader
    image: metric-reader:latest
    env:
    - name: EFS_FILE_SYSTEM_ID
      value: "fs-0123456789abcdef0"
    - name: AWS_REGION
      value: "us-east-1"
    - name: ACTION_PLUGIN
      value: "efs_emergency"
    # ... other environment variables
```

## Usage Example

### Docker Run

```bash
docker run -d \
  -e METRIC_NAME="aws_efs_burst_credit_balance" \
  -e THRESHOLD="<1000000000" \
  -e THRESHOLD_DURATION="5m" \
  -e ACTION_PLUGIN="efs_emergency" \
  -e PLUGIN_DIR="/plugins" \
  -e EFS_FILE_SYSTEM_ID="fs-0123456789abcdef0" \
  -e AWS_REGION="us-east-1" \
  -e AWS_ACCESS_KEY_ID="your-access-key" \
  -e AWS_SECRET_ACCESS_KEY="your-secret-key" \
  -v /path/to/plugins:/plugins \
  metric-reader
```

### Kubernetes Deployment with IRSA

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: metric-reader
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: metric-reader
  template:
    metadata:
      labels:
        app: metric-reader
    spec:
      serviceAccountName: metric-reader
      containers:
      - name: metric-reader
        image: metric-reader:latest
        env:
        - name: METRIC_NAME
          value: "aws_efs_burst_credit_balance"
        - name: LABEL_FILTERS
          value: 'file_system_id="fs-0123456789abcdef0"'
        - name: THRESHOLD
          value: "<1000000000"
        - name: THRESHOLD_DURATION
          value: "5m"
        - name: POLLING_INTERVAL
          value: "30s"
        - name: BACKOFF_DELAY
          value: "1h"
        - name: ACTION_PLUGIN
          value: "efs_emergency"
        - name: PLUGIN_DIR
          value: "/plugins"
        - name: EFS_FILE_SYSTEM_ID
          value: "fs-0123456789abcdef0"
        - name: AWS_REGION
          value: "us-east-1"
        - name: PROMETHEUS_ENDPOINT
          value: "http://prometheus:9090"
        volumeMounts:
        - name: plugins
          mountPath: /plugins
      volumes:
      - name: plugins
        emptyDir: {}
      initContainers:
      - name: copy-plugins
        image: metric-reader:latest
        command: ['sh', '-c', 'cp /app/plugins/*.so /plugins/']
        volumeMounts:
        - name: plugins
          mountPath: /plugins
```

## Cost Considerations

**Important:** Switching from bursting to elastic throughput mode may increase costs. Elastic throughput mode bills based on the amount of throughput used, while bursting mode has a fixed baseline with burst credits.

- Review [AWS EFS Pricing](https://aws.amazon.com/efs/pricing/) before deploying
- Monitor your EFS costs after switching to elastic mode
- Consider the cost trade-off between elastic throughput and potential application downtime from burst credit depletion

## Security Best Practices

1. **Use IRSA on EKS** instead of hardcoding credentials
2. **Apply least privilege** - restrict IAM permissions to specific filesystem ARNs
3. **Enable AWS CloudTrail** to audit EFS API calls
4. **Use separate IAM roles** for different environments (dev, staging, prod)
5. **Rotate credentials regularly** if using access keys (though IRSA is preferred)

## Troubleshooting

### Plugin Fails to Load

- Ensure the plugin is built with the same Go version as the main binary
- Check that `PLUGIN_DIR` points to the correct directory
- Verify the `.so` file has correct permissions

### Authentication Errors

- For IRSA: Verify the service account annotation and IAM role trust policy
- Check AWS CloudTrail logs for detailed error messages
- Test AWS credentials with: `aws sts get-caller-identity`

### Permission Denied Errors

- Verify IAM permissions include `elasticfilesystem:UpdateFileSystem`
- Check that the filesystem ID in the IAM policy matches `EFS_FILE_SYSTEM_ID`
- Ensure the IAM role/user has the policy attached

### Filesystem Update Fails

- Verify the filesystem is in `available` state
- Check that the filesystem exists and the ID is correct
- Some filesystem configurations may prevent throughput mode changes

## Monitoring

The plugin logs detailed information about:
- When the emergency mode is triggered
- The filesystem ID being updated
- The new throughput mode after the update
- Any errors encountered during the operation

Watch the logs with:

```bash
kubectl logs -f deployment/metric-reader
```

## References

- [AWS EFS Throughput Modes](https://docs.aws.amazon.com/efs/latest/ug/performance.html#throughput-modes)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/)
- [EKS IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
- [AWS EFS API Reference](https://docs.aws.amazon.com/efs/latest/ug/API_UpdateFileSystem.html)
