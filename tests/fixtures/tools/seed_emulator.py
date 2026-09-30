"""Seed a local AWS emulator (moto server / Floci) with a few resources.

Only ever run this with AWS_ENDPOINT_URL pointing at a local emulator.
"""
import os
import sys

import boto3

endpoint = os.environ.get("AWS_ENDPOINT_URL", "")
if not endpoint.startswith(("http://localhost", "http://127.0.0.1")):
    sys.exit(f"refusing to seed: AWS_ENDPOINT_URL={endpoint!r} is not a local emulator")

s3 = boto3.client("s3")
for bucket in ["lc7n-public-logs", "lc7n-private-data"]:
    s3.create_bucket(Bucket=bucket)
s3.put_bucket_tagging(
    Bucket="lc7n-private-data",
    Tagging={"TagSet": [{"Key": "owner", "Value": "team-a"}]},
)

ec2 = boto3.client("ec2")
ec2.run_instances(ImageId="ami-12c6146b", MinCount=2, MaxCount=2, InstanceType="t3.micro")
print("seeded")
