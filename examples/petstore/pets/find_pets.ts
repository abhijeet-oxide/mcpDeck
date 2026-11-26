export const tool = {
  name: "find_pets",
  description: "Pets in the Swagger Petstore with one status: id, name, category and tags.",
  useWhen: "Someone asks which pets are available, pending or sold",
  inputSchema: {
    type: "object",
    properties: {
      status: { type: "string", enum: ["available", "pending", "sold"], description: "Pet status" },
      limit: { type: "integer", description: "How many to return, default 20" },
    },
    required: ["status"],
  },
};

export default async function (input: Input) {
  const pets = await petstore.findPetsByStatus({ status: input.status });
  const limit = input.limit ?? 20;
  return {
    total: pets.length,
    shown: Math.min(limit, pets.length),
    pets: pets.slice(0, limit).map((p: any) => ({
      id: p.id,
      name: p.name,
      category: p.category?.name ?? null,
      tags: (p.tags ?? []).map((t: any) => t.name),
    })),
  };
}
