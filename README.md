# imagediff

## What?

Back-references two Docker images to their source code repository and prints the Git changelog between these two images, provided they are annotated with [`label-schema` labels](https://microbadger.com/labels).

## Why?

Unless properly labeled with relevant metadata, Docker containers are complete black boxes.
One can hardly say which version of the code is packaged inside a container, and even less tie a container back to the source code repository...
... unless you label it such information during CI. See https://github.com/weaveworks/build-tools/issues/102 for a discussion on enforcing labels in our build system.

## How?

- Add [`label-schema` labels](https://microbadger.com/labels). The two following labels are required as a bare minimum:

  ```
    LABEL org.label-schema.vcs-ref=$VCS_REF \
          org.label-schema.vcs-url="e.g. https://github.com/microscaling/microscaling"
  ```

- Build Docker images
- Run `imagediff` against these, which will:

    - Pull the Docker images if required.
    - Extract their labels.
    - Extract the VCS' URL and commit hash from the labels.
    - Clone the repository (in-memory).
    - Perform a post-order traversal of the Git history, from the most recent change, to the oldest one, and prints that.

## Example

```bash
$ go run main.go microscaling/microscaling:0.9.0 microscaling/microscaling:0.9.1
45b22cb Merge pull request #40 from microscaling/k8s-labels
91740fb Bump version
309eece Use a separate KubeLabelConfig type for getting labels when using Kubernetes
4756fd6 Get image from k8s deployment object so labels can be retrieved from the MicroBadger API. Move creating the k8s clientset to utils.
```
