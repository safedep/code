package plugin.depsusage.fixtures;

import java.util.List;
import java.util.stream.Collectors;
import com.sun.activation.registries.MailcapFile;
import com.apple.eawt.Application; // unused

import static java.lang.Math.PI;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static java.util.Collections.sort; // unused

import java.util.*;
import java.awt.print.*;
import org.springframework.beans.factory.annotation.*;
import static java.lang.Math.*;

public class testcases {    
    public static void main(String[] args) {
        List<String> items = new ArrayList<>(); 
        System.out.println(Collectors.toList());
        MailcapFile mailcapFile = new MailcapFile();
        
        double circleArea = PI * 25;  // java.lang.Math.PI
        assertEquals(25, circleArea, 0.01);  // org.junit.Assert.assertEquals
        
        // Used via wildcard import -  java.util.*; 
        Set<String> uniqueItems = new HashSet<>();
        
        // @TODO - How to resolve fully qualified package name without any import
        org.slf4j.Logger logger = org.slf4j.LoggerFactory.getLogger(testcases.class);
        logger.info("Using SLF4J logging");
    }
}
