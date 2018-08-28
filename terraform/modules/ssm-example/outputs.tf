# -------------------------------------------------------------------------------
# Output
# -------------------------------------------------------------------------------
output "output_bucket" {
  value = "${aws_s3_bucket.output.id}"
}

output "output_log_group" {
  value = "${aws_cloudwatch_log_group.output.id}"
}
