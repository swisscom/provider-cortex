pipeline {

    agent { label 'docker' }

    tools {
        go 'go-1.19'
    }

    environment {
        V = '0'  // use '1' for verbose build output
        DOCKER_BUILDX_VERSION = 'v0.4.2'
        BUILD_ARGS = '--load' // used in Makefile for docker buildx
        DOCKER_REGISTRY = 'tde-green-public-docker-local.artifactory.swisscom.com'
        REGISTRY_ORGS = 'tde-green-public-docker-local.artifactory.swisscom.com'
        XPKG_REG_ORGS = 'tde-green-public-docker-local.artifactory.swisscom.com'
        DELIVERY_BRANCH = 'master'
        GOPROXY = "https://artifactory.swisscom.com/artifactory/api/go/proxy-golang-go-virtual"
    }

    stages {
        stage('Setup') {
            environment {
                GOPATH = "${env.GOROOT}"
            }
            steps {
                echo 'Setup'
                sh 'printenv'

                setupQEMU()
                setupDockerBuildxPlugin()
                setupGolang()
            }
        }

        stage('Lint') {
            environment {
                GOPATH = "${env.GOROOT}"
            }
            steps {
                sh 'make lint'
            }
        }

        stage('Check Diff') {
            environment {
                GOPATH = "${env.GOROOT}"
            }
            steps {
                sh 'make check-diff'
            }
        }

        stage('Unit Tests') {
            environment {
                GOPATH = "${env.GOROOT}"
            }
            steps {
                sh 'make -j2 test'
            }
        }

        //stage('E2E Tests') {
        //    environment {
        //        GOPATH = "${env.GOROOT}"
        //    }
        //    steps {
        //        sh 'make -j2 build'
        //        sh 'make e2e USE_HELM3=true'
        //    }
        //}

        stage('Build') {
            environment {
                GOPATH = "${env.GOROOT}"
            }
            steps {
                sh 'make -j2 build.all'
            }
        }

        stage('Publish Artifacts to artifactory') {
            when {
                anyOf {
                    branch "${DELIVERY_BRANCH}"
                    branch "release-*"
                }
            }
            steps {
                withCredentials([
                        usernamePassword(credentialsId: 'green-tauri-credentials', passwordVariable: 'TAURI_PASSWORD', usernameVariable: 'TAURI_USERNAME')
                ]) {
                    sh 'docker login -u "${TAURI_USERNAME}" -p "${TAURI_PASSWORD}" "${DOCKER_REGISTRY}"'
                    sh 'make -j2 publish BRANCH_NAME=master'
                }
            }
        }
    }

    post {
        always {
            archiveArtifacts artifacts: "_output/tests/**", allowEmptyArchive: true, caseSensitive: false, fingerprint: false
        }
        cleanup {
            script {
                echo 'cleanup'
                try {
                    sh 'docker buildx uninstall'
                } catch (e) {
                    echo "error when uninstalling docker-buildx: ${e}"
                } finally {
                    cleanWs()
                }
            }
        }
    }
}

// see Customizing section for docker image to use to install: https://github.com/docker/setup-qemu-action
private void setupQEMU() {
    echo 'Setup QEMU'
    sh 'docker run --rm --privileged tonistiigi/binfmt:latest --install all'
}

// see installation steps: https://github.com/docker/buildx
private void setupDockerBuildxPlugin() {
    echo 'Install docker-buildx'
    sh 'curl -sSL https://github.com/docker/buildx/releases/download/$DOCKER_BUILDX_VERSION/buildx-$DOCKER_BUILDX_VERSION.linux-amd64 -o docker-buildx'
    sh 'mkdir -p ~/.docker/cli-plugins/'
    sh 'mv docker-buildx ~/.docker/cli-plugins/'
    sh 'chmod a+x ~/.docker/cli-plugins/docker-buildx'
    sh 'docker buildx install'
}

private void setupGolang() {
    echo 'Prepare go cache dir and load submodules'
    sh 'make go.cachedir'
    sh 'make submodules'

    echo 'Install goimports'
    sh 'go install golang.org/x/tools/cmd/goimports'

    echo 'Download vendor libraries'
    sh 'make vendor vendor.check'
}