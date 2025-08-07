resource "aws_sns_topic" "create_job" {
  name = var.topic_name

  tags = {
    Name        = var.topic_name
    Environment = "Test"
  }
}

output "arn" {
  value = aws_sns_topic.create_job.arn
}