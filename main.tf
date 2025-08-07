module "s3_bucket" {
  source = "./s3"
  bucket_name = "test-image-bucket-e7a7e1da-a8c0-4755-963b-0c2c53501258"
}

module "lambda_create_batch_jobs" {
  source = "./lambdas/lambda_create_batch_jobs"
  lambda_name = "create-batch-jobs"
  s3_arn = module.s3_bucket.arn
  s3_name = module.s3_bucket.name
  sns_create_job_arn = module.sns_create_job.arn
}

module "lambda_child_job" {
  source = "./lambdas/lambda_child_job"
  lambda_name = "child-job"
  s3_name = module.s3_bucket.name
  sns_create_job_arn = module.sns_create_job.arn
  s3_arn = module.s3_bucket.arn
}

module "sns_create_job" {
  source = "./sns/sns-create-job"
  topic_name = "sns-create-job"
}