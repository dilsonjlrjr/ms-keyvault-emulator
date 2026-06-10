package kvemu.compat;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.CommandLineRunner;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class Application implements CommandLineRunner {

    private static final Logger log = LoggerFactory.getLogger(Application.class);

    @Value("${app.cosmos.url:NOT_FOUND}")
    private String cosmosUrl;

    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }

    @Override
    public void run(String... args) {
        log.info("app.cosmos.url = {}", cosmosUrl);
        if (!"https://cosmos.example.com".equals(cosmosUrl)) {
            log.error("ASSERT FAIL: expected https://cosmos.example.com, got {}", cosmosUrl);
        } else {
            log.info("ASSERT OK: app.cosmos.url resolved from COSMO_DB_URL");
        }
    }
}
