resource "aws_lambda_function" "create_batch_jobs" {
  function_name = var.lambda_name
  role          = aws_iam_role.lambda_role.arn
  filename = "${path.module}/lambda/lambda-handler.zip"
  source_code_hash = base64sha256(filebase64("${path.module}/lambda/lambda-handler.zip"))

  runtime = "provided.al2023"
  timeout = "30"
  architectures = ["arm64"]
  handler = "bootstrap"
  memory_size = "128"

  # Advanced logging configuration
  logging_config {
    log_format            = "JSON"
    application_log_level = "INFO"
    system_log_level      = "WARN"
  }

  # Ensure IAM role and log group are ready
  depends_on = [
    aws_iam_role_policy_attachment.lambda_logs,
    aws_cloudwatch_log_group.example
  ]
}

