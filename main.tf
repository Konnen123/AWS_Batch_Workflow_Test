module "s3_bucket" {
  source = "./s3"
  bucket_name = "test-image-bucket-e7a7e1da-a8c0-4755-963b-0c2c53501258"
}

module "lambda_create_batch_jobs" {
  source = "./lambdas/lambda_create_batch_jobs"
  lambda_name = "create-batch-jobs"
  s3_arn = module.s3_bucket.arn
}