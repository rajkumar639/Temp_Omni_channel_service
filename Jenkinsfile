
pipeline {
    agent any

    stages {

        stage('Checkout') {
            steps {
                checkout scm
                echo 'GitHub repository connected successfully!'
            }
        }

        stage('Build WAR') {
            steps {
                bat 'mvn clean package'
            }
        }

        stage('Test') {
            steps {
                echo 'Test stage started'
            }
        }

        stage('Save WAR') {
            steps {
                bat '''
                    if not exist "D:\\Raj\\Restart\\Chain AMdin Temp\\Omni Build War" (
                        mkdir "D:\\Raj\\Restart\\Chain AMdin Temp\\Omni Build War"
                    )

                    copy /Y "target\\*.war" "D:\\Raj\\Restart\\Chain AMdin Temp\\Omni Build War\\"
                '''
            }
        }
    }

    post {
        success {
            echo 'BUILD SUCCESSFUL - WAR file saved!'
        }

        failure {
            echo 'BUILD FAILED!'
        }
    }
}
