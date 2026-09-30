"""Seed a LOCAL AWS emulator (Floci / moto) with demo resources for
examples/policies. Refuses to run against anything but localhost."""
import os
import sys

import boto3

endpoint = os.environ.get("AWS_ENDPOINT_URL", "")
if not endpoint.startswith(("http://localhost", "http://127.0.0.1")):
    sys.exit(f"refusing to seed: AWS_ENDPOINT_URL={endpoint!r} is not a local emulator")

s3 = boto3.client("s3")
for bucket, owner in [("demo-logs", None), ("demo-data", "team-a"), ("demo-tmp", None)]:
    s3.create_bucket(Bucket=bucket)
    if owner:
        s3.put_bucket_tagging(Bucket=bucket, Tagging={"TagSet": [{"Key": "owner", "Value": owner}]})

ec2 = boto3.client("ec2")
ec2.run_instances(
    ImageId="ami-12c6146b", MinCount=2, MaxCount=2, InstanceType="t3.micro",
    TagSpecifications=[{"ResourceType": "instance", "Tags": [{"Key": "owner", "Value": "team-a"}]}],
)
ec2.run_instances(ImageId="ami-12c6146b", MinCount=1, MaxCount=1, InstanceType="t3.small")
for size in (8, 20):
    ec2.create_volume(AvailabilityZone="us-east-1a", Size=size)
print("seeded demo resources")
