@Library('mdblp-library') _
def builderImage
pipeline {
    agent any
    stages {
        stage('Initialization') {
            steps {
                script {
                    utils.initPipeline()
                    if(env.GIT_COMMIT == null) {
                        // git commit id must be a 40 characters length string (lower case or digits)
                        env.GIT_COMMIT = "f".multiply(40)
                    }
                    env.RUN_ID = UUID.randomUUID().toString()
                    docker.image('docker.ci.diabeloop.eu/ci-toolbox').inside() {
                        env.version = sh (
                            script: 'release-helper get-version',
                            returnStdout: true
                        ).trim().toUpperCase()
                    }
                    env.APP_VERSION = env.version
                }
            }
        }
        stage('Build') {
            agent {
                docker {
                    image 'docker.ci.diabeloop.eu/go-build:1.17'
                }
            }
            steps {
                script {
                    withCredentials ([string(credentialsId: 'github-token', variable: 'GITHUB_TOKEN')]) {
                        sh 'git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"'
                        sh "$WORKSPACE/build.sh"
                        sh 'git config --global --unset url."https://${GITHUB_TOKEN}@github.com/".insteadOf'
                    }
                }
            }
        }
        stage('Test') {
            steps {
                echo 'start mongo to serve as a testing db'
                sh 'docker network create hydrotest${RUN_ID} && docker run --rm -d --net=hydrotest${RUN_ID} --name=mongo4hydrotest${RUN_ID} mongo:4.2'
                script {
                    docker.image('docker.ci.diabeloop.eu/go-build:1.17').inside("--net=hydrotest${RUN_ID}") {
                        withCredentials ([string(credentialsId: 'github-token', variable: 'GITHUB_TOKEN')]) {
                            sh 'git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"'
                            sh "TIDEPOOL_STORE_ADDRESSES=mongo4hydrotest${RUN_ID}:27017 TIDEPOOL_STORE_DATABASE=confirm_test $WORKSPACE/test.sh"
                            sh 'git config --global --unset url."https://${GITHUB_TOKEN}@github.com/".insteadOf'
                        }
                    }
                }
            }
            post {
                always {
                    sh 'docker stop mongo4hydrotest${RUN_ID} && docker network rm hydrotest${RUN_ID}'
                    junit 'test-report.xml'
                    archiveArtifacts artifacts: 'coverage.html', allowEmptyArchive: true
                    cobertura coberturaReportFile: 'coverage.xml'
                }
            }
        }
        stage('Package') {
            steps {
                withCredentials ([string(credentialsId: 'github-token', variable: 'GITHUB_TOKEN')]) {
                    pack()
                    sh 'docker build -f Dockerfile.hydromail --build-arg APP_VERSION=$version --build-arg GITHUB_TOKEN=${GITHUB_TOKEN} -t hydromail:${GIT_COMMIT} .'
                }
            }
        }
        stage('Documentation') {
            steps {
                withCredentials ([string(credentialsId: 'github-token', variable: 'GITHUB_TOKEN')]) {
                    genDocumentation()
                }
            }
        }
        stage('Publish') {
            when { branch "dblp" }
            steps {
                publish()
                withCredentials([usernamePassword(credentialsId: 'nexus-jenkins', usernameVariable: 'NEXUS_USER', passwordVariable: 'NEXUS_PWD')]) {
                    pushDocker("${utils.diabeloopRegistry}", "${NEXUS_USER}", "${NEXUS_PWD}", "hydromail:${GIT_COMMIT}", "${version}")
                }
            }
        }
    }
}
