# imagediff

## What?

Back-references two Docker images to their source code repository and prints the Git changelog between these two images, provided they are annotated with:

- [`opencontainers`' `image-spec` labels](https://github.com/opencontainers/image-spec/blob/master/annotations.md#pre-defined-annotation-keys), or
- [`label-schema` labels](https://microbadger.com/labels).

## Why?

Unless properly labeled with relevant metadata, Docker containers are complete black boxes.
One can hardly say which version of the code is packaged inside a container, and even less tie a container back to the source code repository...
... unless you label it such information during CI.

## How?

- Add either:

  - the two following [`opencontainers`' `image-spec` labels](https://github.com/opencontainers/image-spec/blob/master/annotations.md#pre-defined-annotation-keys) to your `Dockerfile`:

    ```
      LABEL org.opencontainers.image.revision=${revision} \
            org.opencontainers.image.source="<URL to your Git repository>"  # e.g.: "https://github.com/microscaling/microscaling"
    ```

    or

  - the two following [`label-schema` labels](https://microbadger.com/labels) ([deprecated](https://github.com/opencontainers/image-spec/blob/master/annotations.md#back-compatibility-with-label-schema)) to your `Dockerfile`:

    ```
      LABEL org.label-schema.vcs-ref=${revision} \
            org.label-schema.vcs-url="<URL to your Git repository>"  # e.g.: "https://github.com/microscaling/microscaling"
    ```

- Parameterise your `Dockerfile` with, e.g.:

  ```
  ARG revision
  ```

- Build your container images, and make sure you inject a value for `revision` when doing so, e.g.:

  ```bash
  docker build --build-arg=revision=$(git rev-parse --short HEAD) .
  ```

- Run `imagediff` against these, which will:

    - Pull the Docker images if required.
    - Extract their labels.
    - Extract the VCS' URL and commit hash from the labels.
    - Clone the repository (in-memory).
    - Perform a post-order traversal of the Git history, from the most recent change, to the oldest one, and print that.

## Example

```bash
$ dep ensure -update
$ go run main.go microscaling/microscaling:0.9.0 microscaling/microscaling:0.9.1
45b22cb Merge pull request #40 from microscaling/k8s-labels
91740fb Bump version
309eece Use a separate KubeLabelConfig type for getting labels when using Kubernetes
4756fd6 Get image from k8s deployment object so labels can be retrieved from the MicroBadger API. Move creating the k8s clientset to utils.
```
