data "aws_iam_policy_document" "assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "lambda_role" {
  name               = "${var.lambda_name}-lambda-role"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

data "aws_iam_policy_document" "lambda_s3_policy" {
  statement {
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:ListBucket",
      "s3:HeadBucket",
    ]
    effect = "Allow"
    resources = [
      "${var.s3_arn}/*",
      var.s3_arn,
    ]
  }
}
resource "aws_iam_role_policy" "lambda_s3_role_policy" {
  policy = data.aws_iam_policy_document.lambda_s3_policy.json
  role   = aws_iam_role.lambda_role.name
}