package kvemu.compat;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.LinkedHashMap;
import java.util.Map;

@RestController
public class ProbeController {

    @Value("${db-password:NOT_FOUND}")
    private String dbPassword;

    @Value("${api-key:NOT_FOUND}")
    private String apiKey;

    @Value("${connection-string:NOT_FOUND}")
    private String connectionString;

    @Value("${app.cosmos.url:NOT_FOUND}")
    private String cosmosUrl;

    @GetMapping("/probe")
    public Map<String, Object> probe() {
        Map<String, Object> result = new LinkedHashMap<>();
        result.put("status", "UP");
        result.put("db_password_loaded",      !dbPassword.equals("NOT_FOUND"));
        result.put("api_key_loaded",           !apiKey.equals("NOT_FOUND"));
        result.put("connection_string_loaded", !connectionString.equals("NOT_FOUND"));
        result.put("cosmos_db_url",            cosmosUrl);

        boolean kvConnected = !dbPassword.equals("NOT_FOUND")
                           && !apiKey.equals("NOT_FOUND")
                           && !connectionString.equals("NOT_FOUND")
                           && "https://cosmos.example.com".equals(cosmosUrl);
        result.put("kv_connected", kvConnected);
        return result;
    }
}
