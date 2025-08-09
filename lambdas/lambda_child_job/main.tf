locals {
  lambda_folder_path = "${path.module}/lambda"
  lambda_main_file_path = "${local.lambda_folder_path}/main.go"
}

resource "aws_lambda_function" "create_batch_jobs" {
  function_name = var.lambda_name
  role          = aws_iam_role.lambda_role.arn
  filename = "${path.module}/lambda/lambda-handler.zip"
  source_code_hash = base64sha256(filebase64(local.lambda_main_file_path))

  runtime = "provided.al2023"
  timeout = "120"
  architectures = ["arm64"]
  handler = "bootstrap"
  memory_size = "128"

  environment {
    variables = {
      BUCKET_NAME = var.s3_name,
      SNS_TOPIC_CHILD_JOB_FINISHED_ARN = var.sns_child_job_finished_arn,
      DYNAMODB_TABLE_NAME = var.dynamodb_table_name
    }
  }

  # Advanced logging configuration
  logging_config {
    log_format            = "JSON"
    application_log_level = "INFO"
    system_log_level      = "WARN"
  }

  # Ensure IAM role and log group are ready
  depends_on = [
    aws_iam_role_policy_attachment.lambda_logs,
    aws_cloudwatch_log_group.example,
    null_resource.build_lambda
  ]
}