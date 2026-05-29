"""Knowledge graph memory tools extracted for MCP."""
import logging
import os
import asyncio
import uuid
from datetime import datetime
from typing import TypedDict, List
import kuzu
from openai import AsyncOpenAI
import config

# Global DB connection
db = None
conn = None
vector_dimension = None

class MemoryRelation(TypedDict):
    head: str
    head_id: str
    relation: str
    tail: str
    tail_id: str

async def get_embedding(text: str) -> List[float]:
    # Always instantiate a new client using the absolute latest config state
    # This prevents the client from locking in stale configs if imported early
    client = AsyncOpenAI(
        api_key=config.EMBEDDING_API_KEY,
        base_url=config.EMBEDDING_BASE_URL,
    )
    
    try:
        response = await client.embeddings.create(
            model=config.EMBEDDING_MODEL,
            input=text,
            encoding_format="float",
        )
        embedding = response.data[0].embedding
        
        # Update vector dimension if not set
        global vector_dimension
        if vector_dimension is None:
            vector_dimension = len(embedding)
            
        return embedding
    except Exception as e:
        error_msg = f"Embedding failed for model '{config.EMBEDDING_MODEL}' at '{config.EMBEDDING_BASE_URL}'. Details: {str(e)}"
        logging.error(error_msg)
        # We raise the error so it bubbles up to the tool response
        raise RuntimeError(error_msg)

def _execute(query: str, parameters: dict | None = None):
    if not conn:
        raise RuntimeError("Database not initialized. Call init_memory_db() first.")
    return conn.execute(query, parameters or {})

async def init_memory_db():
    global db, conn, vector_dimension
    if conn is not None:
        return
    
    # 1. Detect Model Dimension (Once)
    if vector_dimension is None:
        logging.info(f"Detecting vector dimension for model: {config.EMBEDDING_MODEL}")
        sample = await get_embedding("dimension check")
        if sample:
            vector_dimension = len(sample)
        else:
            # Fallback logic based on common models
            model_name = config.EMBEDDING_MODEL.lower()
            if "3-small" in model_name: vector_dimension = 1536
            elif "3-large" in model_name: vector_dimension = 3072
            elif "qwen" in model_name: vector_dimension = 1536 # common for qwen
            else: vector_dimension = 1536 # Default
            logging.warning(f"Detection failed. Using fallback dimension {vector_dimension} for {config.EMBEDDING_MODEL}")

    # 2. Initialize Connection
    db = kuzu.Database(config.KUZU_DB_PATH)
    conn = kuzu.Connection(db)

    # Enable vector search
    try:
        _execute("INSTALL VECTOR;")
        _execute("LOAD VECTOR;")
    except: pass

    # 3. Validate Existing Schema Dimension
    try:
        import re
        res = _execute("CALL TABLE_INFO('Memory') RETURN name, type;")
        existing_dim = None
        while res.has_next():
            row = res.get_next()
            if row[0] == 'embedding':
                match = re.search(r'\[(\d+)\]', row[1])
                if match: existing_dim = int(match.group(1))
                break
        
        if existing_dim and existing_dim != vector_dimension:
            critical_error = (
                f"\n{'!'*60}\n"
                f"DIMENSION MISMATCH ERROR\n"
                f"Model '{config.EMBEDDING_MODEL}' provides {vector_dimension} dimensions,\n"
                f"but your existing database '{config.KUZU_DB_PATH}' was built with {existing_dim} dimensions.\n\n"
                f"TO FIX THIS:\n"
                f"1. Stop the server.\n"
                f"2. Delete the directory/file: '{config.KUZU_DB_PATH}'\n"
                f"3. Restart the server. A new database will be created.\n"
                f"{'!'*60}\n"
            )
            logging.critical(critical_error)
            print(critical_error)
            # We raise an error to stop initialization
            raise RuntimeError("Database dimension mismatch. See logs for fix instructions.")
    except Exception as e:
        if "does not exist" in str(e).lower() or "Table Memory already exists" in str(e):
            pass # Table doesn't exist yet, we'll create it next
        elif "mismatch" in str(e).lower():
            raise # Re-raise our critical error

    # 4. Table Setup
    _execute("CREATE NODE TABLE IF NOT EXISTS User(id STRING, PRIMARY KEY(id));")
    
    try:
        _execute(
            f"CREATE NODE TABLE IF NOT EXISTS Memory("
            f"id STRING, user_id STRING, content STRING, "
            f"embedding FLOAT[{vector_dimension}], created_at STRING, "
            f"PRIMARY KEY(id));"
        )
    except Exception as e:
        if "already exists" not in str(e).lower(): raise

    _execute("CREATE NODE TABLE IF NOT EXISTS Entity(id STRING, name STRING, PRIMARY KEY(id));")
    
    # ... (migration check and rest of the function)
    
    # Migration: check if old schema exists (name as PK, no id)
    try:
        res = _execute("CALL TABLE_INFO('Entity') RETURN name;")
        cols = []
        while res.has_next():
            row = res.get_next()
            if row: cols.append(row[0])
        
        if "id" not in cols:
            logging.warning("Updating Entity table schema to support unique IDs...")
            # Drop dependent tables first
            try:
                _execute("DROP TABLE MENTIONS;")
                # Note: This will also drop any dynamic REL tables that point between Entity nodes
                # Since we don't know their names easily, we might need a more robust migration
                # For now, we drop Entity and assume dynamic rels might need manual cleanup or recreate
                _execute("DROP TABLE Entity;")
                _execute("CREATE NODE TABLE Entity(id STRING, name STRING, PRIMARY KEY(id));")
                _execute("CREATE REL TABLE MENTIONS(FROM Memory TO Entity);")
            except Exception as drop_err:
                logging.error(f"Migration drop failed: {drop_err}")
    except:
        pass

    _execute("CREATE REL TABLE IF NOT EXISTS HAS_MEMORY(FROM User TO Memory);")
    _execute("CREATE REL TABLE IF NOT EXISTS MENTIONS(FROM Memory TO Entity);")

    # Create Index
    try:
        _execute(
            f"CALL CREATE_VECTOR_INDEX('Memory', 'memory_idx', 'embedding', "
            f"metric := 'cosine', cache_embeddings := true);"
        )
    except Exception as e:
        # Ignore if index already exists
        if "already exists" not in str(e).lower():
            logging.warning(f"Could not create vector index: {e}")

async def save_memory(user_id: str, text: str, relations: list[MemoryRelation]) -> str:
    """
    Saves a memory to the graph database and connects it to the user.
    
    This tool stores natural language text as a memory and extracts structured 
    knowledge (entities and relationships) to build a dynamic Knowledge Graph.
    Building this graph allows for complex relationship-based queries later.

    Parameters:
    - user_id: The unique identifier for the user (e.g., "user123").
    - text: The original natural language text stated by the user.
    - relations: A list of dictionaries representing the facts extracted from the text.
      Each dictionary MUST have the following keys:
      - "head": The source entity (e.g., "Alice").
      - "relation": The relationship type in SCREAMING_SNAKE_CASE (e.g., "WORKS_AT", "LIVES_IN").
      - "tail": The target entity (e.g., "Google", "San Francisco").

    Example of 'relations' list:
    [
        {"head": "Alice", "relation": "LIVES_IN", "tail": "New York"},
        {"head": "Alice", "relation": "LIKES", "tail": "Pizza"}
    ]
    """
    await init_memory_db()
    
    vector = await get_embedding(text)

    # Runtime check for dimension mismatch
    global vector_dimension
    if len(vector) != vector_dimension:
        return f"Error: Vector dimension mismatch. Expected {vector_dimension}, got {len(vector)}."

    memory_id = str(uuid.uuid4())
    created_at = datetime.utcnow().isoformat()
    
    try:
        _execute(
            "MERGE (u:User {id: $user_id}) "
            "CREATE (m:Memory {id: $memory_id, user_id: $user_id, content: $text, "
            "embedding: $vector, created_at: $created_at}) "
            "CREATE (u)-[:HAS_MEMORY]->(m);",
            {
                "user_id": str(user_id),
                "memory_id": memory_id,
                "text": text,
                "vector": vector,
                "created_at": created_at,
            },
        )

        for rel in relations:
            head, head_id = rel.get("head"), rel.get("head_id")
            tail, tail_id = rel.get("tail"), rel.get("tail_id")
            rel_type = rel.get("relation")
            
            if not all([head, head_id, tail, tail_id, rel_type]): continue

            # Dynamic Rel table creation
            try:
                _execute(f"CREATE REL TABLE IF NOT EXISTS {rel_type}(FROM Entity TO Entity);")
            except: pass

            _execute("MERGE (h:Entity {id: $head_id}) ON CREATE SET h.name = $head;", {"head_id": head_id, "head": head})
            _execute("MERGE (t:Entity {id: $tail_id}) ON CREATE SET t.name = $tail;", {"tail_id": tail_id, "tail": tail})
            
            _execute(
                f"MATCH (h:Entity {{id: $head_id}}), (t:Entity {{id: $tail_id}}) "
                f"MERGE (h)-[:{rel_type}]->(t);", 
                {"head_id": head_id, "tail_id": tail_id}
            )
            
            _execute(
                "MATCH (m:Memory {id: $memory_id}), (h:Entity {id: $head_id}), (t:Entity {id: $tail_id}) "
                "CREATE (m)-[:MENTIONS]->(h) CREATE (m)-[:MENTIONS]->(t);",
                {"head_id": head_id, "tail_id": tail_id, "memory_id": memory_id}
            )

        return f"Success. Memory saved for User {user_id}."
    except Exception as e:
        if "Runtime error: Cannot insert a vector of size" in str(e):
             return f"Error: Vector dimension mismatch with existing database. {str(e)}"
        return f"Error: {str(e)}"

async def search_memory(query: str, k: int = 5) -> str:
    """Searches across memories using vector similarity."""
    await init_memory_db()
    vector = await get_embedding(query)

    try:
        result = _execute(
            "CALL QUERY_VECTOR_INDEX('Memory', 'memory_idx', $vector, $k) RETURN node.id, distance;",
            {"vector": vector, "k": k}
        )
        
        rows = []
        while result.has_next():
            row = result.get_next()
            if row: rows.append(row)

        if not rows: return "No relevant memories found."

        results = []
        for memory_id, score in rows:
            info = _execute(
                "MATCH (u:User)-[:HAS_MEMORY]->(m:Memory {id: $memory_id}) "
                "OPTIONAL MATCH (m)-[:MENTIONS]->(e:Entity) "
                "RETURN u.id, m.content, collect(e.name), collect(e.id);",
                {"memory_id": memory_id}
            )
            if info.has_next():
                u_id, content, names, ids = info.get_next()
                entities = [f"{n} ({i})" for n, i in zip(names or [], ids or [])]
                results.append(f"- [User {u_id}]: '{content}' (Entities: {', '.join(entities)}) [Score: {score:.2f}]")

        return "\n".join(results)
    except Exception as e:
        return f"Error searching memory: {str(e)}"

async def explore_graph(entity_id: str) -> str:
    """
    Explore the knowledge graph starting from a specific entity ID.
    
    Parameters:
    - entity_id: The UNIQUE identifier for the entity to explore.
    """
    await init_memory_db()
    
    query = (
        "MATCH (e:Entity {id: $id})-[r]->(neighbor:Entity) "
        "RETURN e.name AS s_n, e.id AS s_i, neighbor.name AS t_n, neighbor.id AS t_i, label(r) AS rel, 'OUTGOING' AS dir "
        "UNION ALL "
        "MATCH (neighbor:Entity)-[r]->(e:Entity {id: $id}) "
        "RETURN neighbor.name AS s_n, neighbor.id AS s_i, e.name AS t_n, e.id AS t_i, label(r) AS rel, 'INCOMING' AS dir"
    )
    try:
        result = _execute(query, {"id": entity_id})
        rows = []
        while result.has_next():
            row = result.get_next()
            if row: rows.append(row)
        
        if not rows:
            return f"No relationships found for entity ID '{entity_id}' in the Knowledge Graph."

        outgoing, incoming = [], []
        for s_n, s_i, t_n, t_i, rel, dir in rows:
            line = f"  - ({s_n} [ID: {s_i}]) --[{rel}]--> ({t_n} [ID: {t_i}])"
            if dir == "OUTGOING": outgoing.append(line)
            else: incoming.append(line)
        
        output = [f"Knowledge Graph exploration for entity ID '{entity_id}':"]
        if outgoing:
            output.append("\nOutgoing Relationships (Facts from this entity):")
            output.extend(outgoing)
        if incoming:
            output.append("\nIncoming Relationships (Facts pointing to this entity):")
            output.extend(incoming)
                
        return "\n".join(output)
    except Exception as e:
        return f"Error traversing graph: {str(e)}"

async def explore_graph_deep(entity_id: str) -> str:
    """
    Perform a deep (2-hop) exploration of the knowledge graph starting from an entity ID.
    
    Parameters:
    - entity_id: The UNIQUE identifier for the starting entity.
    """
    await init_memory_db()
    
    query = (
        "MATCH (e:Entity {id: $id})-[r1]->(n1:Entity) "
        "OPTIONAL MATCH (n1)-[r2]->(n2:Entity) "
        "RETURN e.name as e_n, e.id as e_i, label(r1) as r1, n1.name as n1_n, n1.id as n1_i, label(r2) as r2, n2.name as n2_n, n2.id as n2_i"
    )
    
    try:
        result = _execute(query, {"id": entity_id})
        paths = []
        while result.has_next():
            row = result.get_next()
            if row: paths.append(row)
            
        if not paths:
            return f"No deep knowledge found for entity ID '{entity_id}'."
            
        seen = set()
        output = [f"Deep Knowledge Graph exploration (2-hop) for ID '{entity_id}':\n"]
        
        for e_n, e_i, r1, n1_n, n1_i, r2, n2_n, n2_i in paths:
            t1 = (e_i, r1, n1_i)
            if t1 not in seen:
                output.append(f"  - ({e_n} [ID: {e_i}]) --[{r1}]--> ({n1_n} [ID: {n1_i}])")
                seen.add(t1)
            
            if n2_i:
                t2 = (n1_i, r2, n2_i)
                if t2 not in seen:
                    output.append(f"    - ({n1_n} [ID: {n1_i}]) --[{r2}]--> ({n2_n} [ID: {n2_i}])")
                    seen.add(t2)
                    
        return "\n".join(output)
    except Exception as e:
        return f"Error in deep traversal: {str(e)}"

